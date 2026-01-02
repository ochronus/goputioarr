package putio

// ClientAPI defines the methods required to interact with put.io.
// It mirrors the concrete client so it can be mocked in tests.
type ClientAPI interface {
	GetAccountInfo() (*AccountInfoResponse, error)
	ListTransfers() (*ListTransferResponse, error)
	GetTransfer(transferID uint64) (*GetTransferResponse, error)
	RemoveTransfer(transferID uint64) error
	DeleteFile(fileID int64) error
	AddTransfer(url string) error
	UploadFile(data []byte) error
	ListFiles(fileID int64) (*ListFileResponse, error)
	GetFileURL(fileID int64) (string, error)
}
