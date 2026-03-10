package behavior

const (
	BehaviorLogging     = "logging"
	BehaviorErrorHandle = "error-handle"
	BehaviorDatabase    = "database"
	BehaviorHTTPClient  = "http-client"
	BehaviorFileIO      = "file-io"
	BehaviorConcurrency = "concurrency"
)

func AllBehaviors() []string {
	return []string{
		BehaviorLogging,
		BehaviorErrorHandle,
		BehaviorDatabase,
		BehaviorHTTPClient,
		BehaviorFileIO,
		BehaviorConcurrency,
	}
}

func IsValidBehavior(b string) bool {
	for _, behavior := range AllBehaviors() {
		if behavior == b {
			return true
		}
	}
	return false
}
