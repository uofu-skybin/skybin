package metaserver

import (
	"skybin/core"
)

// Interface containing operations that must be implemented for every database backend.
type metaDB interface {
	CloseDB()

	// Renter Operations
	//==================

	// Return a list of all renters in the database
	FindAllRenters() ([]core.RenterInfo, error)
	// Return the renter with the specified ID.
	FindRenterByID(renterID string) (*core.RenterInfo, error)
	// Insert the provided renter into the database.
	InsertRenter(renter *core.RenterInfo) error
	// Update the provided renter in the databse.
	UpdateRenter(renter *core.RenterInfo) error
	// Delete the specified renter from the database.
	DeleteRenter(renterID string) error

	// Provider operations
	//====================

	// Return a list of all the providers in the database.
	FindAllProviders() ([]core.ProviderInfo, error)
	// Return the provider with the specified ID.
	FindProviderByID(providerID string) (*core.ProviderInfo, error)
	// Insert the given provider into the database.
	InsertProvider(provider *core.ProviderInfo) error
	// Update the given provider in the databse.
	UpdateProvider(provider *core.ProviderInfo) error
	// Delete the specified provider from the dtabase.
	DeleteProvider(providerID string) error

	// File operations
	//====================

	// Return a list of all files in the database.
	FindAllFiles() ([]core.File, error)
	// Return the latest version of the file with the specified ID.
	FindFileByID(fileID string) (*core.File, error)
	// Return a list of files present in the renter's directory.
	FindFilesInRenterDirectory(renterID string) ([]core.File, error)
	// Return a map of names to files shared with a given renter.
	FindFilesSharedWithRenter(renterID string) ([]core.File, error)
	// Return a list of files that the renter owns.
	FindFilesByOwner(renterID string) ([]core.File, error)
	// Insert the given file into the database.
	InsertFile(file *core.File) error
	// Update the given file in the database.
	UpdateFile(file *core.File) error
	// Delete all versions of the given file from the database.
	DeleteFile(fileID string) error

	// Contract operations
	//=====================

	// Return a list of all contracts in the database.
	FindAllContracts() ([]core.Contract, error)
	// Return the contract with the specified ID
	FindContractByID(contractID string) (*core.Contract, error)
	// Return a list of contracts belonging to the specified renter.
	FindContractsByRenter(renterID string) ([]core.Contract, error)
	// Insert the given contract into the database.
	InsertContract(contract *core.Contract) error
	// Update the given contract.
	UpdateContract(contract *core.Contract) error
	// Delete the contract.
	DeleteContract(contractID string) error
}
