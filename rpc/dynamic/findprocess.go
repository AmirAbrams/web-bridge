package dynamic

import (
	"fmt"
	"strconv"
	"strings"

	ps "github.com/shirou/gopsutil/process"
)

// FindDynamicdProcess returns the dynamicd processes running locally
func FindDynamicdProcess() (*ps.Process, error) {
	processList, err := ps.Processes()
	if err != nil {
		return nil, fmt.Errorf("ps.Processes() Failed")
	}
	for _, process := range processList {
		name, _ := process.Name()
		if strings.HasPrefix(name, DefaultName) {
			//util.Info.Println("Running dynamicd process found", name)
			cmd, _ := process.Cmdline()
			// TODO check datadir as well
			if strings.Index(cmd, "-port="+strconv.Itoa(int(DefaultPort))) > 0 && strings.Index(cmd, "-rpcport="+strconv.Itoa(int(DefaultRPCPort))) > 0 {
				return process, nil
			}
			//util.Info.Println("Different dynamicd process found", cmd)
		}
	}
	return nil, fmt.Errorf("Running dynamicd process not found")
}
