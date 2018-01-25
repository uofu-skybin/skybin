package metaserver

import (
	"skybin/core"
	"time"
)

// DB internal representation of core data structures
//===================================================

type RenterInfo struct {
	ID        string       `json:"id"`
	PublicKey string       `json:"publicKey"`
	Files     []FileRecord `json:"files"`
	Shared    []FileRecord `json:"shared"`
}

type Version struct {
	ID     string       `json:"id"`
	Blocks []core.Block `json:"blocks"`
}

type File struct {
	ID         string            `json:"id"`
	IsDir      bool              `json:"isDir"`
	Size       int64             `json:"size"`
	ModTime    time.Time         `json:"modTime"`
	AccessList []core.Permission `json:"accessList"`
	Versions   []Version         `json:"versions"`
	OwnerID    string            `json:"ownerID"`
}

type FileRecord struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Interface containing operations that must be implemented for every database backend.
type metaDB interface {
	// Renter Operations
	//==================

	// Return a list of all renters in the database
	FindAllRenters() ([]RenterInfo, error)
	// Return the renter with the specified ID.
	FindRenterByID(renterID string) (*RenterInfo, error)
	// Insert the provided renter into the database.
	InsertRenter(renter RenterInfo) error
	// Update the provided renter in the databse.
	UpdateRenter(renter RenterInfo) error
	// Delete the specified renter from the database.
	DeleteRenter(renterID string) error

	// Provider operations
	//====================

	// Return a list of all the providers in the database.
	FindAllProviders() ([]core.ProviderInfo, error)
	// Return the provider with the specified ID.
	FindProviderByID(providerID string) (*core.ProviderInfo, error)
	// Insert the given provider into the database.
	InsertProvider(provider core.ProviderInfo) error
	// Update the given provider in the databse.
	UpdateProvider(provider core.ProviderInfo) error
	// Delete the specified provider from the dtabase.
	DeleteProvider(providerID string) error

	// File operations
	//====================

	// Return a list of all files in the database.
	FindAllFiles() ([]File, error)
	// Return the latest version of the file with the specified ID.
	FindFileByID(fileID string) (*File, error)
	// Return a list of files present in the renter's directory.
	FindFilesByRenter(renterID string) ([]File, error)
	// Insert the given file into the database.
	InsertFile(file File) error
	// Update the given fiel in the database.
	UpdateFile(file File) error
	// Delete all versions of the given file from the database.
	DeleteFile(fileID string) error
}
