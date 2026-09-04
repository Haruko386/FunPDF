package common

import (
	"fmt"

	"go.uber.org/zap"
)

const banner = "\n" +
	"								   ,-.----.                           \n" +
	"    ,---,.                         \\    /  \\      ,---,        ,---,. \n" +
	"  ,'  .' |                         |   :    \\   .'  .' `\\    ,'  .' | \n" +
	",---.'   |         ,--,      ,---, |   |  .\\ :,---.'     \\ ,---.'   | \n" +
	"|   |   .'       ,'_ /|  ,-+-. /  |.   :  |: ||   |  .`\\  ||   |   .' \n" +
	":   :  :    .--. |  | : ,--.'|'   ||   |   \\ ::   : |  '  |:   |  :   \n" +
	":   |  |-,,'_ /| :  . ||   |  ,\"' ||   : .   /|   | '  ;  ::   |  |-, \n" +
	"|   :  ;/||  ' | |  . .|   | /  | |;   | |`-' '   | ;  .  ||   :  ;/| \n" +
	"|   |   .'|  | ' |  | ||   | |  | ||   | ;    |   | :  |  '|   |   .' \n" +
	"'   :  '  :  | : ;  ; ||   | |  |/ :   | |    '   : | /  ; '   :  '   \n" +
	"|   |  |  '  :  `--'   \\   | |--'  :   : :    |   | '` ,/  |   |  |   \n" +
	"|   :  \\  :  ,      .-./   |/      |   | :    ;   :  .'    |   :  \\   \n" +
	"|   | ,'   `--`----'   '---'       `---'.|    |   ,.'      |   | ,'   \n" +
	"`----'                               `---`    '---'        `----'       \n"

var (
	logger *zap.Logger
)

// SyncLog flushes buffered log entries.
func SyncLog() {
	if logger != nil {
		_ = logger.Sync()
	}
}

// Fatal writes a fatal log entry and terminates the process.
func Fatal(msg string, fields ...zap.Field) {
	if logger == nil {
		panic("logger not initialized")
	}
	logger.Fatal(msg, fields...)
}

// Info writes an informational log entry.
func Info(msg string, fields ...zap.Field) {
	if logger == nil {
		return
	}
	logger.Info(msg, fields...)
}

// Debug writes a debug log entry.
func Debug(msg string, fields ...zap.Field) {
	if logger == nil {
		return
	}
	logger.Debug(msg, fields...)
}

// Warn writes a warning log entry.
func Warn(msg string, fields ...zap.Field) {
	if logger == nil {
		return
	}
	logger.Warn(msg, fields...)
}

// Banner prints the ASCII art logo to stdout.
func Banner() {
	fmt.Print(banner)
}
