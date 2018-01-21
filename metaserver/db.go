package metaserver

import (
	"skybin/core"
)

//
type metaDB interface {
	// Renter Operations
	//==================

	// Return a list of all renters in the database
	FindAllRenters() []core.RenterInfo
	// Return the renter with the specified ID.
	FindRenterByID(renterID string) (core.RenterInfo, error)
	// Insert the provided renter into the database.
	InsertRenter(renter core.RenterInfo) error
	// Update the provided renter in the databse.
	UpdateRenter(renter core.RenterInfo) error
	// Delete the specified renter from the database.
	DeleteRenter(renterID string) error

	// Provider operations
	//====================

	// Return a list of all the providers in the database.
	FindAllProviders() []core.ProviderInfo
	// Return the provider with the specified ID.
	FindProviderByID(providerID string) (core.ProviderInfo, error)
	// Insert the given provider into the database.
	InsertProvider(provider core.ProviderInfo) error
	// Update the given provider in the databse.
	UpdateProvider(provider core.ProviderInfo) error
	// Delete the specified provider from the dtabase.
	DeleteProvider(providerID string) error

	// File operations
	//====================

	// Return a list of all files in the database.
	FindAllFiles() []core.File
	// Return the latest version of the file with the specified ID.
	FindFileByID(fileID string) (core.File, error)
	// Return the specified version of the file with the given ID.
	FindFileByIDAndVersion(fileID string, version int) (core.File, error)
	// Insert the given file into the database.
	InsertFile(file core.File) error
	// Update the given fiel in the database.
	UpdateFile(file core.File) error
	// Delete all versions of the given file from the database.
	DeleteFile(fileID string) error
	// Delete file with the given version and ID.
	DeleteFileVersion(fileID string, version int) error
}
