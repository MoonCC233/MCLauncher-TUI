package install

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/mirror"
	"github.com/MoonCC233/MCLauncher-TUI/internal/mojang"
	"github.com/MoonCC233/MCLauncher-TUI/internal/paths"
	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

type EventType string

const (
	EventStage    EventType = "stage"
	EventProgress EventType = "progress"
	EventLog      EventType = "log"
	EventDone     EventType = "done"
)

type Event struct {
	Type       EventType
	Stage      string
	Detail     string
	Done       int64
	Total      int64
	FilesDone  int
	FilesTotal int
}

type Installer struct {
	Layout     paths.Layout
	VersionID  string
	Manifest   *mojang.Manifest
	Threads    int
	Client     *http.Client
	SkipClient bool // keep an existing (patched) client jar
	OnEvent    func(Event)
}

type InstalledMarker struct {
	VersionID   string `json:"version_id"`
	InstalledAt int64  `json:"installed_at"`
	AssetIndex  string `json:"asset_index"`
	Assets      int    `json:"assets"`
	Libraries   int    `json:"libraries"`
}

func IsInstalled(layout paths.Layout, versionID string) bool {
	if versionID == "" {
		return false
	}
	if _, err := os.Stat(layout.Marker(versionID)); err != nil {
		return false
	}
	if _, err := os.Stat(layout.VersionJSON(versionID)); err != nil {
		return false
	}
	jarID := versionID
	if v, err := mojang.LoadVersionJSON(layout, versionID); err == nil {
		jarID = v.ClientJarID(versionID)
	}
	info, err := os.Stat(layout.ClientJar(jarID))
	return err == nil && info.Size() > 0
}

func InstalledSet(layout paths.Layout) map[string]bool {
	out := map[string]bool{}
	entries, err := os.ReadDir(layout.Versions)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if entry.IsDir() && IsInstalled(layout, entry.Name()) {
			out[entry.Name()] = true
		}
	}
	return out
}

func (in *Installer) emit(ev Event) {
	if in.OnEvent != nil {
		in.OnEvent(ev)
	}
}

func (in *Installer) threads() int {
	if in.Threads > 0 {
		return util.Clamp(in.Threads, 1, 32)
	}
	return 8
}

type fileTask struct {
	dest  string
	url   string
	sha1  string
	size  int64
	label string
	kind  string
}

func (in *Installer) Install(ctx context.Context) error {
	client := in.Client
	if client == nil {
		client = util.NewHTTPClient()
	}
	in.emit(Event{Type: EventStage, Stage: "Preparing", Detail: "Resolving version metadata"})
	version, err := mojang.FetchVersionJSON(ctx, client, in.Layout, in.Manifest, in.VersionID, false)
	if err != nil {
		return err
	}
	if version.MainClass == "" {
		return fmt.Errorf("version %s has no main class", in.VersionID)
	}

	assetID := version.AssetID()
	var assetIndex *mojang.AssetIndexFile
	if version.AssetIndex != nil && version.AssetIndex.URL != "" {
		in.emit(Event{Type: EventStage, Stage: "Preparing", Detail: "Downloading asset index"})
		indexPath := in.Layout.AssetIndex(assetID)
		need := true
		if util.VerifyFile(indexPath, version.AssetIndex.SHA1, version.AssetIndex.Size) {
			need = false
		}
		if need {
			err := util.DownloadFile(ctx, client, util.DownloadSpec{
				URL:  version.AssetIndex.URL,
				Dest: indexPath,
				SHA1: version.AssetIndex.SHA1,
				Size: version.AssetIndex.Size,
			}, nil)
			if err != nil {
				return fmt.Errorf("asset index: %w", err)
			}
		}
		idx, err := mojang.ReadAssetIndex(in.Layout, assetID)
		if err != nil {
			return fmt.Errorf("parse asset index %s: %w", assetID, err)
		}
		assetIndex = idx
	} else if _, err := os.Stat(in.Layout.AssetIndex(assetID)); err == nil {
		idx, idxErr := mojang.ReadAssetIndex(in.Layout, assetID)
		if idxErr == nil {
			assetIndex = idx
		}
	}

	features := mojang.ArgFeatures(0, 0)
	res := mojang.ResolveLibraries(version, features)

	tasks := map[string]*fileTask{}
	addTask := func(t *fileTask) {
		if t.dest == "" {
			return
		}
		if _, exists := tasks[t.dest]; exists {
			return
		}
		tasks[t.dest] = t
	}

	jarID := version.ClientJarID(in.VersionID)
	clientPath := in.Layout.ClientJar(jarID)
	if in.SkipClient {
		if st, statErr := os.Stat(clientPath); statErr != nil || st.Size() == 0 {
			return fmt.Errorf("client jar for %s is missing", in.VersionID)
		}
	} else {
		clientInfo := version.Downloads.Client
		if !util.VerifyFile(clientPath, clientInfo.SHA1, clientInfo.Size) {
			if clientInfo.URL == "" {
				return fmt.Errorf("version %s has no client download URL", in.VersionID)
			}
			addTask(&fileTask{
				dest:  clientPath,
				url:   clientInfo.URL,
				sha1:  clientInfo.SHA1,
				size:  clientInfo.Size,
				label: filepath.Base(clientPath),
				kind:  "client",
			})
		}
	}
	for _, artifact := range res.Artifacts {
		if artifact.URL == "" {
			continue
		}
		addTask(&fileTask{
			dest:  mojang.ArtifactPath(in.Layout, artifact),
			url:   artifact.URL,
			sha1:  artifact.SHA1,
			size:  artifact.Size,
			label: filepath.Base(artifact.Path),
			kind:  "library",
		})
	}
	for _, native := range res.Natives {
		if native.URL == "" {
			continue
		}
		addTask(&fileTask{
			dest:  mojang.NativePath(in.Layout, native),
			url:   native.URL,
			sha1:  native.SHA1,
			size:  native.Size,
			label: filepath.Base(native.Path),
			kind:  "library",
		})
	}
	if version.Logging != nil && version.Logging.Client != nil && version.Logging.Client.File.URL != "" {
		info := version.Logging.Client.File
		addTask(&fileTask{
			dest:  logConfigPath(in.Layout, version),
			url:   info.URL,
			sha1:  info.SHA1,
			size:  info.Size,
			label: filepath.Base(info.URL),
			kind:  "library",
		})
	}
	assetCount := 0
	if assetIndex != nil {
		for name, obj := range assetIndex.Objects {
			if obj.Hash == "" || len(obj.Hash) < 2 || obj.Size <= 0 {
				continue
			}
			assetCount++
			addTask(&fileTask{
				dest:  in.Layout.AssetObject(obj.Hash),
				url:   "https://resources.download.minecraft.net/" + obj.Hash[:2] + "/" + obj.Hash,
				sha1:  obj.Hash,
				size:  obj.Size,
				label: name,
				kind:  "asset",
			})
		}
	}

	var pending []*fileTask
	var alreadyBytes int64
	for _, t := range tasks {
		info, statErr := os.Stat(t.dest)
		if statErr == nil && !info.IsDir() {
			ok := false
			if t.kind == "asset" {
				ok = t.size > 0 && info.Size() == t.size
			} else {
				ok = util.VerifyFile(t.dest, t.sha1, t.size)
			}
			if ok {
				alreadyBytes += t.size
				continue
			}
		}
		pending = append(pending, t)
	}

	var pendingBytes int64
	for _, t := range pending {
		pendingBytes += t.size
	}
	if alreadyBytes > 0 && len(pending) > 0 {
		in.emit(Event{Type: EventLog, Stage: "Reusing installed files", Detail: util.HumanBytes(alreadyBytes) + " already present"})
	}
	in.emit(Event{
		Type:       EventStage,
		Stage:      "Downloading game files",
		Detail:     fmt.Sprintf("%d files · %s", len(pending), util.HumanBytes(pendingBytes)),
		Total:      pendingBytes,
		FilesTotal: len(pending),
	})
	if len(pending) > 0 {
		if err := in.downloadAll(ctx, client, pending, pendingBytes); err != nil {
			return err
		}
	}

	in.emit(Event{Type: EventStage, Stage: "Extracting natives", Detail: "Preparing native libraries"})
	if err := mojang.ExtractNatives(in.Layout, in.VersionID, res); err != nil {
		return err
	}

	if assetIndex != nil && assetIndex.Virtual {
		in.emit(Event{Type: EventStage, Stage: "Preparing legacy assets", Detail: assetID})
		if err := materializeVirtual(ctx, in.Layout, assetID, assetIndex); err != nil {
			return err
		}
	}

	in.emit(Event{Type: EventStage, Stage: "Finishing", Detail: "Writing installation marker"})
	marker := InstalledMarker{
		VersionID:   in.VersionID,
		InstalledAt: time.Now().Unix(),
		AssetIndex:  assetID,
		Assets:      assetCount,
		Libraries:   len(res.Artifacts),
	}
	if err := util.WriteJSONAtomic(in.Layout.Marker(in.VersionID), &marker); err != nil {
		return err
	}
	in.emit(Event{Type: EventDone, Stage: "Ready", Done: pendingBytes, Total: pendingBytes, FilesDone: len(pending), FilesTotal: len(pending)})
	return nil
}

type progressTracker struct {
	mu         sync.Mutex
	completed  int64
	active     map[int]int64
	total      int64
	filesDone  int
	filesTotal int
	lastEmit   time.Time
	lastDetail string
	onEvent    func(Event)
	stage      string
}

func (p *progressTracker) progress(idx int, n int64, size int64) {
	p.mu.Lock()
	if size > 0 && n > size {
		n = size
	}
	p.active[idx] = n
	now := time.Now()
	if now.Sub(p.lastEmit) < 120*time.Millisecond {
		p.mu.Unlock()
		return
	}
	p.lastEmit = now
	done := p.completed
	for _, v := range p.active {
		done += v
	}
	if done > p.total {
		done = p.total
	}
	ev := Event{
		Type:       EventProgress,
		Stage:      p.stage,
		Detail:     p.lastDetail,
		Done:       done,
		Total:      p.total,
		FilesDone:  p.filesDone,
		FilesTotal: p.filesTotal,
	}
	p.mu.Unlock()
	if p.onEvent != nil {
		p.onEvent(ev)
	}
}

func (p *progressTracker) setDetail(detail string) {
	p.mu.Lock()
	p.lastDetail = detail
	p.mu.Unlock()
}

func (p *progressTracker) finish(idx int, size int64) {
	p.mu.Lock()
	delete(p.active, idx)
	if size > 0 {
		p.completed += size
	}
	if p.completed > p.total {
		p.completed = p.total
	}
	p.filesDone++
	done := p.completed
	filesDone := p.filesDone
	p.lastEmit = time.Now()
	ev := Event{
		Type:       EventProgress,
		Stage:      p.stage,
		Detail:     p.lastDetail,
		Done:       done,
		Total:      p.total,
		FilesDone:  filesDone,
		FilesTotal: p.filesTotal,
	}
	p.mu.Unlock()
	if p.onEvent != nil {
		p.onEvent(ev)
	}
}

func (in *Installer) downloadAll(ctx context.Context, client *http.Client, tasks []*fileTask, totalBytes int64) error {
	workers := in.threads()
	if workers > len(tasks) {
		workers = len(tasks)
	}
	tracker := &progressTracker{
		active:     map[int]int64{},
		total:      totalBytes,
		filesTotal: len(tasks),
		onEvent:    in.OnEvent,
		stage:      "Downloading game files",
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan int)
	var (
		wg       sync.WaitGroup
		firstErr error
		errMu    sync.Mutex
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if workCtx.Err() != nil {
					return
				}
				task := tasks[idx]
				tracker.setDetail(task.label)
				urls := mirror.Candidates(task.url)
				err := util.DownloadFile(workCtx, client, util.DownloadSpec{
					URL:  urls[0],
					Alt:  urls[1:],
					Dest: task.dest,
					SHA1: task.sha1,
					Size: task.size,
				}, func(n int64) { tracker.progress(idx, n, task.size) })
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("%s: %w", task.label, err)
					}
					errMu.Unlock()
					cancel()
					return
				}
				tracker.finish(idx, task.size)
			}
		}()
	}
feed:
	for i := range tasks {
		select {
		case <-workCtx.Done():
			break feed
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	errMu.Lock()
	defer errMu.Unlock()
	if firstErr != nil {
		return firstErr
	}
	for _, task := range tasks {
		if info, err := os.Stat(task.dest); err != nil || info.Size() != task.size {
			if task.size == 0 {
				if _, statErr := os.Stat(task.dest); statErr == nil {
					continue
				}
			}
			return fmt.Errorf("download did not complete: %s", task.label)
		}
	}
	return nil
}

func logConfigPath(layout paths.Layout, version *mojang.VersionJSON) string {
	name := ""
	if version.Logging != nil && version.Logging.Client != nil {
		info := version.Logging.Client.File
		if info.Path != "" {
			name = path.Base(strings.ReplaceAll(info.Path, "\\", "/"))
		} else if info.URL != "" {
			name = path.Base(strings.SplitN(info.URL, "?", 2)[0])
		}
	}
	if name == "" || name == "." || name == "/" {
		name = version.EffectiveID("client") + "-log4j.xml"
	}
	return filepath.Join(layout.LogConfigs, name)
}

func LogConfigPath(layout paths.Layout, version *mojang.VersionJSON) string {
	return logConfigPath(layout, version)
}

const consolePatternLayout = `<PatternLayout pattern="[%d{HH:mm:ss}] [%t/%level]: %msg{nolookups}%n" />`

var xmlLogLayoutPattern = regexp.MustCompile(`(?is)<(?:Legacy)?XMLLayout\s*/>`)

func ConsoleLogConfig(layout paths.Layout, source string) (string, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	if !xmlLogLayoutPattern.Match(data) {
		return source, nil
	}
	patched := xmlLogLayoutPattern.ReplaceAll(data, []byte(consolePatternLayout))
	name := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	dest := filepath.Join(layout.Cache, "log_configs", name+"-console.xml")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if existing, readErr := os.ReadFile(dest); readErr == nil && string(existing) == string(patched) {
		return dest, nil
	}
	if err := os.WriteFile(dest, patched, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func materializeVirtual(ctx context.Context, layout paths.Layout, assetID string, index *mojang.AssetIndexFile) error {
	target := layout.VirtualAssets(assetID)
	if _, err := os.Stat(filepath.Join(target, ".mclauncher-ok")); err == nil {
		return nil
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	count := 0
	for name, obj := range index.Objects {
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if count%100 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		src := layout.AssetObject(obj.Hash)
		dest, err := util.SafeJoin(target, name)
		if err != nil {
			return fmt.Errorf("legacy asset name %q: %w", name, err)
		}
		if st, err := os.Stat(dest); err == nil && st.Size() == obj.Size {
			continue
		}
		if err := util.EnsureDir(filepath.Dir(dest)); err != nil {
			return err
		}
		if err := copyFile(src, dest); err != nil {
			return fmt.Errorf("copy legacy asset %s: %w", name, err)
		}
	}
	return util.WriteJSONAtomic(filepath.Join(target, ".mclauncher-ok"), map[string]any{
		"index": assetID,
		"count": len(index.Objects),
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// EnsureGameAssets returns the correct ${game_assets} path for a version and
// materialises map_to_resources indexes inside the instance game directory.
func EnsureGameAssets(ctx context.Context, layout paths.Layout, version *mojang.VersionJSON, gameDir string) (string, error) {
	assetID := version.AssetID()
	idx, err := mojang.ReadAssetIndex(layout, assetID)
	if err != nil {
		// Versions without an asset index simply point at the assets root.
		return layout.Assets, nil
	}
	if idx.Virtual {
		virtual := layout.VirtualAssets(assetID)
		if _, statErr := os.Stat(filepath.Join(virtual, ".mclauncher-ok")); statErr == nil {
			return virtual, nil
		}
		return layout.Assets, nil
	}
	if !idx.MapToResources {
		return layout.Assets, nil
	}
	target := filepath.Join(gameDir, "resources")
	marker := filepath.Join(gameDir, ".mclauncher", "resources-"+assetID+".ok")
	if _, err := os.Stat(marker); err == nil {
		return target, nil
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}
	count := 0
	for name, obj := range idx.Objects {
		count++
		if count%100 == 0 {
			if err := ctx.Err(); err != nil {
				return "", err
			}
		}
		dest, err := util.SafeJoin(target, name)
		if err != nil {
			return "", err
		}
		if st, err := os.Stat(dest); err == nil && st.Size() == obj.Size {
			continue
		}
		if err := util.EnsureDir(filepath.Dir(dest)); err != nil {
			return "", err
		}
		if err := copyFile(layout.AssetObject(obj.Hash), dest); err != nil {
			return "", err
		}
	}
	if err := util.EnsureDir(filepath.Dir(marker)); err != nil {
		return "", err
	}
	if err := util.WriteJSONAtomic(marker, map[string]any{"index": assetID, "count": count}); err != nil {
		return "", err
	}
	return target, nil
}

var ErrCancelled = errors.New("installation cancelled")
