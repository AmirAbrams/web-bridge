package dynamic

import (
	"time"

	util "github.com/duality-solutions/web-bridge/internal/utilities"
)

// WaitForSync waits for the Dynamic blockchain to fully sync
func (d *Dynamicd) WaitForSync(stopchan chan struct{}, checkDelaySeconds, endDelaySeconds uint16) bool {
	status, _ := d.GetSyncStatus()
	for status.SyncProgress < 1 {
		select {
		default:
			time.Sleep(time.Duration(checkDelaySeconds) * time.Second)
			status, _ = d.GetSyncStatus()
		case <-stopchan:
			util.Info.Println("WaitForSync stopped")
			return false
		}
	}
	time.Sleep(time.Duration(endDelaySeconds) * time.Second)
	return true
}
