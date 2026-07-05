package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var (
	flagVideoSize        string
	flagVideoDescription string
	flagVideoDuration    int
)

var activeScreenRecorder *engine.ScreenRecorder

var videoStartCmd = &cobra.Command{
	Use:   "video-start [filename]",
	Short: "Start video recording with frame capture",
	Long: `Start video recording for the active session. Captures JPEG frames via CDP
screencast and saves them to disk. video-stop writes a manifest and reports
the number of frames captured.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		b, page := openPage()
		defer b.Close()
		if !b.Connected() {
			exitErr("video-start", fmt.Errorf("requires -s/--session, --connect, or an attached default session"))
		}
		if state := b.VideoState(); state.Active {
			exitErr("video-start", fmt.Errorf("video already active since %s", state.StartedAt))
		}
		filename := "video.webm"
		if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
			filename = args[0]
		}
		framesDir := playwrightArtifactPath("frames")
		rec := engine.NewScreenRecorder(page, engine.ScreenRecorderOpts{
			OutputDir: framesDir,
		})
		if err := rec.Start(); err != nil {
			exitErr("video-start", err)
		}
		activeScreenRecorder = rec

		state := engine.VideoState{
			Active:    true,
			StartedAt: time.Now().UTC().Format(time.RFC3339),
			Filename:  playwrightArtifactPath(filepath.Base(filename)),
			Size:      flagVideoSize,
		}
		if err := b.SetVideoState(state); err != nil {
			exitErr("video-start", err)
		}
		output(map[string]any{
			"active":    true,
			"filename":  state.Filename,
			"framesDir": framesDir,
			"size":      state.Size,
		}, fmt.Sprintf("video recording started → frames in %s", framesDir))
	},
}

var videoChapterCmd = &cobra.Command{
	Use:   "video-chapter <title>",
	Short: "Add a video chapter marker",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		b, _ := openPage()
		defer b.Close()
		if !b.Connected() {
			exitErr("video-chapter", fmt.Errorf("requires -s/--session, --connect, or an attached default session"))
		}
		state := b.VideoState()
		if !state.Active {
			exitErr("video-chapter", fmt.Errorf("video metadata is not active"))
		}
		atMs := elapsedVideoMs(state.StartedAt, time.Now().UTC())
		chapter := engine.VideoChapter{
			Title:       args[0],
			Description: flagVideoDescription,
			DurationMs:  flagVideoDuration,
			AtMs:        atMs,
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		state.Chapters = append(state.Chapters, chapter)
		if err := b.SetVideoState(state); err != nil {
			exitErr("video-chapter", err)
		}
		output(chapter, fmt.Sprintf("video chapter added: %s", chapter.Title))
	},
}

var videoStopCmd = &cobra.Command{
	Use:   "video-stop",
	Short: "Stop video recording and write manifest",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		b, _ := openPage()
		defer b.Close()
		if !b.Connected() {
			exitErr("video-stop", fmt.Errorf("requires -s/--session, --connect, or an attached default session"))
		}
		state := b.VideoState()
		if !state.Active {
			exitErr("video-stop", fmt.Errorf("video is not active"))
		}

		var frameCount int64
		framesDir := playwrightArtifactPath("frames")
		if activeScreenRecorder != nil {
			frameCount = activeScreenRecorder.Stop()
			framesDir = activeScreenRecorder.OutputDir()
			activeScreenRecorder = nil
		}

		manifestPath := videoManifestPath(state.Filename)
		if err := writeVideoManifest(manifestPath, state, time.Now().UTC(), frameCount, framesDir); err != nil {
			exitErr("video-stop", err)
		}
		if err := b.SetVideoState(engine.VideoState{}); err != nil {
			exitErr("video-stop", err)
		}
		type videoStopResult struct {
			Manifest  string `json:"manifest"`
			Filename  string `json:"filename"`
			Chapters  int    `json:"chapters"`
			Frames    int64  `json:"frames_captured"`
			FramesDir string `json:"frames_dir"`
		}
		result := videoStopResult{
			Manifest:  manifestPath,
			Filename:  state.Filename,
			Chapters:  len(state.Chapters),
			Frames:    frameCount,
			FramesDir: framesDir,
		}
		text := fmt.Sprintf("video stopped: %d frames captured in %s", frameCount, framesDir)
		output(result, text)
	},
}

var flagVideoShowDuration int
var flagVideoShowPosition string
var flagVideoShowCursor string

var videoShowActionsCompatCmd = &cobra.Command{
	Use:   "video-show-actions",
	Short: "Display action markers during video capture (unsupported in ghostchrome)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		unsupportedPlaywrightCommand("video-show-actions", args, "ghostchrome does not emit action overlays in its video metadata", "Use your runner’s existing trace/video tooling for action overlays.")
	},
}

var videoHideActionsCompatCmd = &cobra.Command{
	Use:   "video-hide-actions",
	Short: "Hide action markers during video capture (unsupported in ghostchrome)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		unsupportedPlaywrightCommand("video-hide-actions", args, "ghostchrome does not emit action overlays in its video metadata", "Use your runner’s existing trace/video tooling for action overlays.")
	},
}

func elapsedVideoMs(startedAt string, now time.Time) int64 {
	started, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return 0
	}
	if now.Before(started) {
		return 0
	}
	return now.Sub(started).Milliseconds()
}

func videoManifestPath(filename string) string {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	if base == "" || base == "." {
		base = "video"
	}
	return playwrightArtifactPath(base + ".video.json")
}

func writeVideoManifest(path string, state engine.VideoState, stoppedAt time.Time, frameCount int64, framesDir string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload := map[string]any{
		"filename":              state.Filename,
		"requested_size":        state.Size,
		"auto_started":          state.Auto,
		"source":                state.Source,
		"started_at":            state.StartedAt,
		"stopped_at":            stoppedAt.UTC().Format(time.RFC3339),
		"chapters":              state.Chapters,
		"frames_captured":       frameCount,
		"frames_dir":            framesDir,
		"playwright_compatible": false,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func autoStartVideoIfConfigured(b *engine.Browser) {
	if flagAutoVideoSize == "" || b == nil || !b.Connected() {
		return
	}
	if state := b.VideoState(); state.Active {
		return
	}
	started := time.Now().UTC()
	state := engine.VideoState{
		Active:    true,
		StartedAt: started.Format(time.RFC3339),
		Filename:  playwrightArtifactPath("video-" + started.Format("20060102T150405Z") + ".webm"),
		Size:      flagAutoVideoSize,
		Auto:      true,
		Source:    flagAutoVideoSource,
	}
	if err := b.SetVideoState(state); err != nil {
		fmt.Fprintf(os.Stderr, "warning: auto video metadata not started: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[video] metadata auto-started from %s (%s)\n", flagAutoVideoSource, flagAutoVideoSize)
}

func init() {
	videoStartCmd.Flags().StringVar(&flagVideoSize, "size", "", "Requested video size, e.g. 800x600")
	videoChapterCmd.Flags().StringVar(&flagVideoDescription, "description", "", "Chapter description")
	videoChapterCmd.Flags().IntVar(&flagVideoDuration, "duration", 0, "Chapter card duration in milliseconds")
	videoShowActionsCompatCmd.Flags().IntVar(&flagVideoShowDuration, "duration", 500, "Compatibility passthrough flag (unsupported; kept for API parity)")
	videoShowActionsCompatCmd.Flags().StringVar(&flagVideoShowPosition, "position", "top-right", "Compatibility passthrough flag (unsupported; kept for API parity)")
	videoShowActionsCompatCmd.Flags().StringVar(&flagVideoShowCursor, "cursor", "pointer", "Compatibility passthrough flag (unsupported; kept for API parity)")
	rootCmd.AddCommand(videoStartCmd, videoChapterCmd, videoStopCmd, videoShowActionsCompatCmd, videoHideActionsCompatCmd)
	commandGroups["video-start"] = "observe"
	commandGroups["video-chapter"] = "observe"
	commandGroups["video-stop"] = "observe"
	commandGroups["video-show-actions"] = "observe"
	commandGroups["video-hide-actions"] = "observe"
}
