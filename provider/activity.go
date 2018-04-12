package provider

import "fmt"

type activity struct {
	Timestamps          []string `json:"timestamps"`
	BlockUploads        []int64  `json:"blockUploads"`
	BlockDownloads      []int64  `json:"blockDownloads"`
	BlockDeletions      []int64  `json:"blockDeletions"`
	BytesUploaded       []int64  `json:"bytesUploaded"`
	BytesDownloaded     []int64  `json:"bytesDownloaded"`
	StorageReservations []int64  `json:"storageReservations"`
}

type recents struct {
	Hour *summary `json:"hour"`
	Day  *summary `json:"day"`
	Week *summary `json:"week"`
}

type summary struct {
	BlockUploads        int64 `json:"blockUploads"`
	BlockDownloads      int64 `json:"blockDownloads"`
	BlockDeletions      int64 `json:"blockDeletions"`
	StorageReservations int64 `json:"storageReservations"`
}

const (
	activityOpUpload = "upload"
	activityOpDownload = "download"
	activityOpDelete = "delete"
	activityOpContract = "contract"
)

func (provider *Provider) addActivity(op string, bytes int64) error {
	err := provider.db.CycleActivity()
	if err != nil {
		return fmt.Errorf("Error adding new activity to DB: %s", err)
	}

	if op == activityOpUpload {
		err = provider.db.UpdateActivity("BlockUploads", 1)
		if err != nil {
			return fmt.Errorf("add upload activity failed. error: %s", err)
		}
		err = provider.db.UpdateActivity("BytesUploaded", bytes)
		if err != nil {
			return fmt.Errorf("add upload activity failed. error: %s", err)
		}
	} else if op == activityOpDownload {
		err = provider.db.UpdateActivity("BlockDownloads", 1)
		if err != nil {
			return fmt.Errorf("add download activity failed. error: %s", err)
		}
		err = provider.db.UpdateActivity("BytesDownloaded", bytes)
		if err != nil {
			return fmt.Errorf("add download activity failed. error:  %s", err)
		}
	} else if op == activityOpDelete {
		provider.db.UpdateActivity("BlockDeletions", 1)
		if err != nil {
			return fmt.Errorf("add delete activity failed. error:  %s", err)
		}

	} else if op == activityOpContract {
		provider.db.UpdateActivity("StorageReservations", 1)
		if err != nil {
			return fmt.Errorf("add contract activity failed. error: %s", err)
		}

	}
	return nil
}
