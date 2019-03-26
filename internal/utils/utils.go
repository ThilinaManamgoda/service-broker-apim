package utils

import (
	"code.cloudfoundry.org/lager"
	"fmt"
	"github.com/pkg/errors"
	"github.com/wso2/service-broker-apim/internal/constants"
	"io"
	"os"
)

// InitLogger initializes lager logging object
// 1. Setup log level
// 2. Setup log file
func InitLogger(logFile, logLevelS string) (lager.Logger, error) {
	logLevel, err := lager.LogLevelFromString(logLevelS)
	if err != nil {
		return nil, errors.Wrapf(err, constants.ErrMsgInvalidLogLevel, logLevel)
	}
	f, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, constants.FilePerm)
	if err != nil {
		return nil, errors.Wrapf(err, constants.ErrMsgUnableToOpenLogFile, logFile)
	}
	logger := lager.NewLogger(constants.LoggerName)
	logger.RegisterSink(lager.NewWriterSink(io.MultiWriter(os.Stdout, f), logLevel))
	return logger, nil
}

// HandleErrorAndExit prints an error and exit with exit code 1
// Only applicable upto server startup since process will be killed once invoked
func HandleErrorAndExit(err error) {
	fmt.Println(err)
	os.Exit(1)
}

// HandleErrorWithLoggerAndExit prints an error through the provided logger and exit with exit code 1
// Only applicable upto server startup since process will be killed once invoked
func HandleErrorWithLoggerAndExit(logger lager.Logger, errMsg string, err error) {
	logger.Error(errMsg, err)
	os.Exit(1)
}
