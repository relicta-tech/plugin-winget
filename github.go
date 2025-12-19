package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	wingetPkgsOwner = "microsoft"
	wingetPkgsRepo  = "winget-pkgs"
	githubAPIBase   = "https://api.github.com"
)

// GitHubClient handles GitHub API operations for winget-pkgs.
type GitHubClient struct {
	token     string
	forkOwner string
	client    *http.Client
}

// NewGitHubClient creates a new GitHub client.
func NewGitHubClient(token, forkOwner string) *GitHubClient {
	return &GitHubClient{
		token:     token,
		forkOwner: forkOwner,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// EnsureFork ensures the user has a fork of winget-pkgs.
func (g *GitHubClient) EnsureFork(ctx context.Context) (string, error) {
	// If fork owner is specified, use it
	if g.forkOwner != "" {
		return g.forkOwner, nil
	}

	// Get current user
	user, err := g.getCurrentUser(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	// Check if fork exists
	exists, err := g.forkExists(ctx, user)
	if err != nil {
		return "", fmt.Errorf("failed to check fork: %w", err)
	}

	if exists {
		return user, nil
	}

	// Create fork
	if err := g.createFork(ctx); err != nil {
		return "", fmt.Errorf("failed to create fork: %w", err)
	}

	// Wait for fork to be ready
	time.Sleep(5 * time.Second)

	return user, nil
}

// CreatePR creates a pull request with the manifests.
func (g *GitHubClient) CreatePR(ctx context.Context, manifests *ManifestSet, cfg PRConfig) (string, error) {
	forkOwner := g.forkOwner
	if forkOwner == "" {
		user, err := g.getCurrentUser(ctx)
		if err != nil {
			return "", err
		}
		forkOwner = user
	}

	// Get base branch SHA
	baseSHA, err := g.getBranchSHA(ctx, wingetPkgsOwner, wingetPkgsRepo, cfg.BaseBranch)
	if err != nil {
		return "", fmt.Errorf("failed to get base branch SHA: %w", err)
	}

	// Create branch name
	branchName := fmt.Sprintf("winget/%s/%s",
		strings.ReplaceAll(manifests.Version.PackageIdentifier, ".", "-"),
		manifests.Version.PackageVersion)

	// Create branch in fork
	if err := g.createBranch(ctx, forkOwner, branchName, baseSHA); err != nil {
		return "", fmt.Errorf("failed to create branch: %w", err)
	}

	// Get files to commit
	files, err := manifests.GetFiles()
	if err != nil {
		return "", fmt.Errorf("failed to get manifest files: %w", err)
	}

	// Commit files
	commitMessage := fmt.Sprintf("New version: %s version %s",
		manifests.Version.PackageIdentifier, manifests.Version.PackageVersion)

	if err := g.commitFiles(ctx, forkOwner, branchName, files, commitMessage); err != nil {
		return "", fmt.Errorf("failed to commit files: %w", err)
	}

	// Create PR
	prTitle := renderTemplate(cfg.Title, map[string]string{
		"PackageId": manifests.Version.PackageIdentifier,
		"Version":   manifests.Version.PackageVersion,
	})

	prURL, err := g.createPullRequest(ctx, forkOwner, branchName, cfg.BaseBranch, prTitle)
	if err != nil {
		return "", fmt.Errorf("failed to create PR: %w", err)
	}

	return prURL, nil
}

func (g *GitHubClient) getCurrentUser(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubAPIBase+"/user", nil)
	if err != nil {
		return "", err
	}

	var result struct {
		Login string `json:"login"`
	}

	if err := g.doRequest(req, &result); err != nil {
		return "", err
	}

	return result.Login, nil
}

func (g *GitHubClient) forkExists(ctx context.Context, owner string) (bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", githubAPIBase, owner, wingetPkgsRepo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := g.doRequestRaw(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK, nil
}

func (g *GitHubClient) createFork(ctx context.Context) error {
	url := fmt.Sprintf("%s/repos/%s/%s/forks", githubAPIBase, wingetPkgsOwner, wingetPkgsRepo)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := g.doRequestRaw(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create fork: %s", string(body))
	}

	return nil
}

func (g *GitHubClient) getBranchSHA(ctx context.Context, owner, repo, branch string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/ref/heads/%s", githubAPIBase, owner, repo, branch)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	var result struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}

	if err := g.doRequest(req, &result); err != nil {
		return "", err
	}

	return result.Object.SHA, nil
}

func (g *GitHubClient) createBranch(ctx context.Context, owner, branch, sha string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/git/refs", githubAPIBase, owner, wingetPkgsRepo)

	body := map[string]string{
		"ref": "refs/heads/" + branch,
		"sha": sha,
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	resp, err := g.doRequestRaw(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create branch: %s", string(respBody))
	}

	return nil
}

func (g *GitHubClient) commitFiles(ctx context.Context, owner, branch string, files map[string]string, message string) error {
	// For each file, create or update it
	for path, content := range files {
		url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", githubAPIBase, owner, wingetPkgsRepo, path)

		body := map[string]string{
			"message": message,
			"content": base64.StdEncoding.EncodeToString([]byte(content)),
			"branch":  branch,
		}

		jsonBody, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(jsonBody))
		if err != nil {
			return err
		}

		resp, err := g.doRequestRaw(req)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to create file %s: status %d", path, resp.StatusCode)
		}
	}

	return nil
}

func (g *GitHubClient) createPullRequest(ctx context.Context, forkOwner, branch, baseBranch, title string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls", githubAPIBase, wingetPkgsOwner, wingetPkgsRepo)

	body := map[string]string{
		"title": title,
		"head":  fmt.Sprintf("%s:%s", forkOwner, branch),
		"base":  baseBranch,
		"body":  "This PR was automatically created by Relicta.",
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}

	var result struct {
		HTMLURL string `json:"html_url"`
	}

	if err := g.doRequest(req, &result); err != nil {
		return "", err
	}

	return result.HTMLURL, nil
}

func (g *GitHubClient) doRequest(req *http.Request, result any) error {
	resp, err := g.doRequestRaw(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func (g *GitHubClient) doRequestRaw(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return g.client.Do(req)
}
