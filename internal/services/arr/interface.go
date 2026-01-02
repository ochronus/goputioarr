package arr

// ClientAPI defines the minimal Arr client contract used by the rest of the app.
// It enables mocking Arr interactions in tests without hitting real services.
type ClientAPI interface {
	CheckImported(targetPath string) (bool, error)
}
