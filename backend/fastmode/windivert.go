package fastmode

import (
	"context"
	"log/slog"
	"strings"
)

// windivertServiceName is the kernel service WinDivert self-registers when
// winws.exe opens it. The app never creates it (winws/WinDivert.dll does), so
// nothing in the normal lifecycle deletes it — it lingers as a driver artifact
// after Fast Mode stops, and survives a hard kill. Reset & Quit and the
// uninstaller remove it explicitly.
const windivertServiceName = "WinDivert"

// RemoveWinDivertService stops and deletes the WinDivert kernel service if it
// is registered. Best-effort: a not-installed service (sc error 1060) is
// treated as success, since "already gone" is the desired end state. This must
// only be called when Fast Mode is not running — a live winws.exe holds the
// driver open and would block deletion (and re-create it on next launch).
func RemoveWinDivertService(log *slog.Logger) error {
	ctx := context.Background()
	// Stop first (ignore result — it may already be stopped or absent), then
	// delete. sc reports 1060 "service does not exist" when there's nothing to
	// remove, which we treat as clean.
	_, _ = runCommand(ctx, "sc", "stop", windivertServiceName)
	out, err := runCommand(ctx, "sc", "delete", windivertServiceName)
	if err != nil {
		lower := strings.ToLower(out)
		if strings.Contains(lower, "1060") || strings.Contains(lower, "does not exist") {
			return nil
		}
		if log != nil {
			log.Warn("could not delete WinDivert service (may require the driver to be idle)", "output", strings.TrimSpace(out), "error", err)
		}
		return err
	}
	if log != nil {
		log.Info("removed WinDivert driver service")
	}
	return nil
}
