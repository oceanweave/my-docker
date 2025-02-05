package container

import (
	log "github.com/sirupsen/logrus"
	"os"
)

func Run(tty bool, cmd string) {
	parent := NewParentProcess(tty, cmd)
	if err := parent.Start(); err != nil {
		log.Error(err)
	}
	_ = parent.Wait()
	os.Exit(-1)
}
