//go:build manual

package java

import (
	"context"
	"testing"
	"time"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

func TestManualInstallJava(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	lastStage := ""
	lastLog := time.Time{}
	info, err := InstallLatest(ctx, util.NewHTTPClient(), t.TempDir(), 21, func(p Progress) {
		if p.Stage != lastStage || time.Since(lastLog) > 5*time.Second {
			t.Logf("%s · %s · %d/%d", p.Stage, p.Detail, p.Done, p.Total)
			lastStage = p.Stage
			lastLog = time.Now()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("installed %s at %s (%s)", info.Label(), info.Path, info.Vendor)
}
