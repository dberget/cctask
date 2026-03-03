package tui

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

func generateProofHTML(projectRoot string, task *model.Task) (string, error) {
	dir := store.ScreenshotsDir(projectRoot)

	beforeData, err := os.ReadFile(filepath.Join(dir, task.ProofBefore))
	if err != nil {
		return "", fmt.Errorf("read before screenshot: %w", err)
	}
	afterData, err := os.ReadFile(filepath.Join(dir, task.ProofAfter))
	if err != nil {
		return "", fmt.Errorf("read after screenshot: %w", err)
	}

	beforeB64 := base64.StdEncoding.EncodeToString(beforeData)
	afterB64 := base64.StdEncoding.EncodeToString(afterData)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Proof: %s</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { background: #1a1a2e; color: #e0e0e0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; }
  .header { padding: 24px 32px; text-align: center; border-bottom: 1px solid #333; }
  .header h1 { font-size: 20px; font-weight: 600; }
  .header .task-id { color: #888; font-size: 14px; margin-top: 4px; }
  .container { display: flex; gap: 2px; padding: 16px; height: calc(100vh - 90px); }
  .panel { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
  .label { text-align: center; padding: 8px; font-weight: 600; font-size: 14px; text-transform: uppercase; letter-spacing: 1px; }
  .label-before { background: #3d1f1f; color: #ff6b6b; }
  .label-after { background: #1f3d1f; color: #6bff6b; }
  .image-wrap { flex: 1; overflow: auto; background: #111; padding: 8px; }
  .image-wrap img { width: 100%%; height: auto; display: block; }
</style>
</head>
<body>
<div class="header">
  <h1>%s</h1>
  <div class="task-id">%s</div>
</div>
<div class="container">
  <div class="panel">
    <div class="label label-before">Before</div>
    <div class="image-wrap"><img src="data:image/png;base64,%s" alt="Before"></div>
  </div>
  <div class="panel">
    <div class="label label-after">After</div>
    <div class="image-wrap"><img src="data:image/png;base64,%s" alt="After"></div>
  </div>
</div>
</body>
</html>`, task.Title, task.Title, task.ID, beforeB64, afterB64)

	return html, nil
}

func openProofInBrowser(projectRoot string, task *model.Task) error {
	html, err := generateProofHTML(projectRoot, task)
	if err != nil {
		return err
	}

	dir := store.ScreenshotsDir(projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	htmlPath := filepath.Join(dir, task.ID+"-proof.html")
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		return err
	}

	return exec.Command("open", htmlPath).Start()
}
