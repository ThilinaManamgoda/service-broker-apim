/*
 *  Copyright (c) 2019, WSO2 Inc. (http://www.wso2.org) All Rights Reserved.
 */

// Package utils holds a common set of Util functions
package utils

import (
	"code.cloudfoundry.org/lager"
	"fmt"
	"github.com/pkg/errors"
	"github.com/wso2/service-broker-apim/pkg/constants"
	"io"
	"os"
)

var logger lager.Logger
var ioWriter io.Writer

// GetEnv returns the value (which may be empty) If the Key is present in the environment
// Otherwise the default value is returned
func GetEnv(key, defaultVal string) string {
	val, exists := os.LookupEnv(key)
	if exists {
		return val
	}
	return defaultVal
}

// InitLogger initializes lager logging object
// 1. Setup log level
// 2. Setup log file
// Must initialize logger object to handle logging
func InitLogger(logFile, logLevelS string) (lager.Logger, error) {
	logL, err := lager.LogLevelFromString(logLevelS)
	if err != nil {
		return nil, errors.Wrapf(err, constants.ErrMsgInvalidLogLevel, logL)
	}
	f, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, constants.FilePerm)
	if err != nil {
		return nil, errors.Wrapf(err, constants.ErrMsgUnableToOpenLogFile, logFile)
	}
	logger = lager.NewLogger(constants.LoggerName)
	ioWriter = io.MultiWriter(os.Stdout, f)
	logger.RegisterSink(lager.NewWriterSink(ioWriter, logL))
	return logger, nil
}

// IoWriterLog returns the IO writer object for logging
func IoWriterLog() io.Writer{
	return ioWriter
}


// LogInfo logs Info level messages using configured lager.Logger
func LogInfo(msg string) {
	logger.Info(msg)
}

// LogError logs Info level messages using configured lager.Logger
func LogError(msg string, err error) {
	logger.Error(msg, err)
}

// LogDebug logs Info level messages using configured lager.Logger
func LogDebug(msg string) {
	logger.Debug(msg)
}

// HandleErrorAndExit prints an error and exit with exit code 1
// Only applicable upto server startup since process will be killed once invoked
func HandleErrorAndExit(err error) {
	fmt.Println(err)
	os.Exit(constants.ExitCode1)
}

// HandleErrorWithLoggerAndExit prints an error through the provided logger and exit with exit code 1
// Only applicable upto server startup since process will be killed once invoked
func HandleErrorWithLoggerAndExit(errMsg string, err error) {
	LogError(errMsg, err)
	os.Exit(constants.ExitCode1)
}


// ValidateParams returns false if one of the arguments are empty
func ValidateParams(vals ...string) bool{
	if vals == nil {
		return false
	}
	for _, val := range vals {
		if val == "" {
			return false
		}
	}
	return true
}
