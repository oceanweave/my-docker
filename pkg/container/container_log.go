package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
)

func LogContainer(containerId string) {
	logFilePath := fmt.Sprintf(ContainerInfoPathFormat, containerId) + LogFile
	file, err := os.Open(logFilePath)
	defer file.Close()
	if err != nil {
		log.Errorf("Log container open file %s error %v", logFilePath, err)
		return
	}
	content, err := io.ReadAll(file)
	if err != nil {
		log.Errorf("Log container read file %s error %v", logFilePath, err)
		return
	}
	_, err = fmt.Fprintln(os.Stdout, string(content))
	if err != nil {
		log.Errorf("Log container Fprint error %v", err)
		return
	}
}
