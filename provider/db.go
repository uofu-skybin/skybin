package provider

import (
	"database/sql"
	"fmt"
	"skybin/core"
	"time"

	_ "github.com/mattn/go-sqlite3" //sqlite library
)

// Inherit from sql.DB so we can add associated methods
type ProviderDB struct {
	*sql.DB
}

// Initialize DB
func SetupDB(path string) (*ProviderDB, error) {
	// Open DB
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("Failed to open DB. error: %s", err)
	}

	// Create contracts table
	stmt, err := db.Prepare(`CREATE TABLE IF NOT EXISTS contracts ( id INTEGER PRIMARY KEY, 
		ContractId TEXT, 
		RenterId TEXT, 
		ProviderId TEXT, 
		StorageSpace INTEGER, 
		StartDate TEXT, 
		EndDate TEXT, 
		RenterSignature TEXT, 
		ProviderSignature TEXT,
		StorageFee INTEGER)
		`)
	if err != nil {
		return nil, fmt.Errorf("Failed to prepare contracts table. error: %s", err)
	}
	_, err = stmt.Exec()
	if err != nil {
		return nil, fmt.Errorf("Failed to create contracts table. error: %s", err)
	}

	// Create blocks table
	stmt, err = db.Prepare(`CREATE TABLE IF NOT EXISTS blocks ( id INTEGER PRIMARY KEY, 
		RenterId TEXT, 
		BlockId TEXT, 
		Size INTEGER)`)
	if err != nil {
		return nil, fmt.Errorf("Failed to prepare blocks table. error: %s", err)
	}
	_, err = stmt.Exec()
	if err != nil {
		return nil, fmt.Errorf("Failed to create blocks table. error: %s", err)
	}

	// Create activity table
	stmt, err = db.Prepare(`CREATE TABLE IF NOT EXISTS activity ( id INTEGER PRIMARY KEY, 
		Period TEXT, 
		Timestamp TEXT, 
		BlockUploads INTEGER DEFAULT 0, 
		BlockDownloads INTEGER DEFAULT 0, 
		BlockDeletions INTEGER DEFAULT 0, 
		BytesUploaded INTEGER DEFAULT 0, 
		BytesDownloaded INTEGER DEFAULT 0, 
		StorageReservations INTEGER DEFAULT 0 )`)
	if err != nil {
		return nil, fmt.Errorf("Failed to prepare activity table. error: %s", err)
	}
	_, err = stmt.Exec()
	if err != nil {
		return nil, fmt.Errorf("Failed to create activity table. error: %s", err)
	}

	// Add blockid index on blocks
	stmt, err = db.Prepare(`CREATE INDEX IF NOT EXISTS blockid_blocks ON blocks (BlockId)`)

	if err != nil {
		return nil, fmt.Errorf("Failed to prepare Index for BlockId on Blocks table. error: %s", err)
	}
	_, err = stmt.Exec()
	if err != nil {
		return nil, fmt.Errorf("Failed to create Index for BlockId on Blocks table. error: %s", err)
	}

	// Add renterid index on blocks
	stmt, err = db.Prepare(`CREATE INDEX IF NOT EXISTS renterid_blocks ON blocks (RenterId)`)

	if err != nil {
		return nil, fmt.Errorf("Failed to prepare Index for RenterId on Blocks table. error: %s", err)
	}
	_, err = stmt.Exec()
	if err != nil {
		return nil, fmt.Errorf("Failed to create Index for RenterId on Blocks table. error: %s", err)
	}

	// Add crenterid index on contracts
	stmt, err = db.Prepare(`CREATE INDEX IF NOT EXISTS renterid_idx ON contracts (RenterId)`)

	if err != nil {
		return nil, fmt.Errorf("Failed to prepare Index for contracts. error: %s", err)
	}
	_, err = stmt.Exec()
	if err != nil {
		return nil, fmt.Errorf("Failed to create Index for contracts. error: %s", err)
	}
	pdb := &ProviderDB{db}
	return pdb, nil
}

// Insert new activity if activity doesn't exist on that interval
// Intervals correspond to:
// - hour: 12 5-minute intervals
// - day: 24 1-hour intervals
// - hour: 12 5-minute intervals
func (db *ProviderDB) InsertActivity() error {
	t := time.Now()
	hour := t.Truncate(time.Minute * 5).Format(time.RFC3339)
	day := t.Truncate(time.Hour).Format(time.RFC3339)
	week := t.Truncate(time.Hour * 24).Format(time.RFC3339)

	stmt, err := db.Prepare(`INSERT INTO activity (Period, Timestamp) 
		Select 'hour', ? WHERE NOT EXISTS(
		SELECT 1 FROM activity WHERE Period = 'hour' and Timestamp = ?)`)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(hour, hour)
	if err != nil {
		return err
	}

	stmt, err = db.Prepare(`INSERT INTO activity (Period, Timestamp) 
		Select 'day', ? WHERE NOT EXISTS(
		SELECT 1 FROM activity WHERE Period = 'day' and Timestamp = ? )`)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(day, day)
	if err != nil {
		return err
	}

	stmt, err = db.Prepare(`INSERT INTO activity (Period, Timestamp) 
		Select 'week', ? WHERE NOT EXISTS(
		SELECT 1 FROM activity WHERE Period = 'week' and Timestamp = ? )`)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(week, week)
	if err != nil {
		return err
	}

	return nil
}

// Increment activity corresponding to interval and operation by value
func (db *ProviderDB) UpdateActivity(op string, value int64) error {
	query := fmt.Sprintf(`UPDATE activity SET %s = %s + ? 
		WHERE (Timestamp = ? and Period = 'hour') 
		or (Timestamp = ? and Period = 'day') 
		or (Timestamp = ? and Period = 'week')`, op, op)

	// Truncate to sub-intervals as follows
	// hour: 12 5-minute intervals
	// day: 24 1-hour intervals
	// hour: 12 5-minute intervals
	t := time.Now()
	hour := t.Truncate(time.Minute * 5).Format(time.RFC3339)
	day := t.Truncate(time.Hour).Format(time.RFC3339)
	week := t.Truncate(time.Hour * 24).Format(time.RFC3339)

	stmt, err := db.Prepare(query)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(value, hour, day, week)
	if err != nil {
		return err
	}
	return nil
}

// Drop activity that is no longer in scope
func (db *ProviderDB) DeleteActivity() error {
	stmt, err := db.Prepare(`DELETE from activity WHERE Period='hour' and Timestamp < ?`)
	t := time.Now().Add(-1 * time.Hour)
	_, err = stmt.Exec(t.Format(time.RFC3339))
	if err != nil {
		return err
	}

	stmt, err = db.Prepare(`DELETE from activity WHERE Period='day' and Timestamp < ?`)
	t = time.Now().Add(-1 * time.Hour * 24)
	_, err = stmt.Exec(t.Format(time.RFC3339))
	if err != nil {
		return err
	}

	stmt, err = db.Prepare(`DELETE from activity WHERE Period='week' and Timestamp < ?`)
	t = time.Now().Add(-1 * time.Hour * 24 * 7)
	_, err = stmt.Exec(t.Format(time.RFC3339))
	if err != nil {
		return err
	}
	return nil
}

// This is called by the local provider server on GET /stats
func (db *ProviderDB) GetStatsResp() (*getStatsResp, error) {
	query := fmt.Sprintf(`SELECT Period, Timestamp, BlockUploads, 
		BlockDownloads, BlockDeletions, BytesUploaded, 
		BytesDownloaded, StorageReservations FROM activity`)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	resp := makeStatsResp()
	var period string
	var timestamp string
	var blockUploads int64
	var blockDownloads int64
	var blockDeletions int64
	var bytesUploaded int64
	var bytesDownloaded int64
	var storageReservations int64

	for rows.Next() {
		// scan does not parse these directly into time.Time correctly
		err = rows.Scan(&period, &timestamp, &blockUploads, &blockDownloads, &blockDeletions, &bytesUploaded, &bytesDownloaded, &storageReservations)
		if err != nil {
			return nil, err
		}
		// On day we are also concerned with information to populate charts
		if period == "day" {
			resp.RecentSummary.Day.BlockUploads += blockUploads
			resp.RecentSummary.Day.BlockDownloads += blockDownloads
			resp.RecentSummary.Day.BlockDeletions += blockDeletions
			resp.RecentSummary.Day.StorageReservations += storageReservations

			stamp, err := time.Parse(time.RFC3339, timestamp)
			if err != nil {
				return nil, err
			}

			// Increment activity counters for the day interval
			idx := 23 - int(time.Since(stamp).Hours())
			if idx < 24 && idx > 0 {
				resp.ActivityCounter.BlockUploads[idx] += blockUploads
				resp.ActivityCounter.BlockDownloads[idx] += blockDownloads
				resp.ActivityCounter.BlockDeletions[idx] += blockDeletions
				resp.ActivityCounter.BytesUploaded[idx] += bytesUploaded
				resp.ActivityCounter.BytesDownloaded[idx] += bytesDownloaded
				resp.ActivityCounter.StorageReservations[idx] += storageReservations
			}
		} else if period == "hour" {
			resp.RecentSummary.Hour.BlockUploads += blockUploads
			resp.RecentSummary.Hour.BlockDownloads += blockDownloads
			resp.RecentSummary.Hour.BlockDeletions += blockDeletions
			resp.RecentSummary.Hour.StorageReservations += storageReservations
		} else if period == "week" {
			resp.RecentSummary.Week.BlockUploads += blockUploads
			resp.RecentSummary.Week.BlockDownloads += blockDownloads
			resp.RecentSummary.Week.BlockDeletions += blockDeletions
			resp.RecentSummary.Week.StorageReservations += storageReservations
		}
	}
	return resp, nil
}

func (db *ProviderDB) InsertContract(contract *core.Contract) error {
	stmt, err := db.Prepare(`INSERT INTO contracts 
		(ContractId, RenterId, ProviderId, StorageSpace, 
		StartDate, EndDate, RenterSignature, ProviderSignature, StorageFee) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(contract.ID,
		contract.RenterId,
		contract.ProviderId,
		contract.StorageSpace,
		contract.StartDate.Format(time.RFC3339),
		contract.EndDate.Format(time.RFC3339),
		contract.RenterSignature,
		contract.ProviderSignature,
		contract.StorageFee,
	)
	if err != nil {
		return err
	}
	return nil
}

// Currently unused, but probably relevant for canceling contracts
func (db *ProviderDB) DeleteContractById(contractId string) error {
	stmt, err := db.Prepare(`DELETE from contracts where ContractId=?`)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(contractId)
	if err != nil {
		return err
	}
	return nil
}

// This is used in GET /renter-info
func (db *ProviderDB) GetContractsByRenter(renterId string) ([]*core.Contract, error) {
	query := fmt.Sprintf(`SELECT ContractId, RenterId, ProviderId, StorageSpace, 
		RenterSignature, ProviderSignature, StorageFee, StartDate, EndDate 
		FROM contracts where RenterId='%s'`, renterId)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	var contracts []*core.Contract

	for rows.Next() {
		c := &core.Contract{}
		// scan does not parse these directly into time.Time correctly
		var startDate string
		var endDate string
		err = rows.Scan(&c.ID, &c.RenterId, &c.ProviderId, &c.StorageSpace, &c.RenterSignature, &c.ProviderSignature, &c.StorageFee, &startDate, &endDate)
		if err != nil {
			return nil, err
		}
		c.StartDate, _ = time.Parse(time.RFC3339, startDate)
		c.EndDate, _ = time.Parse(time.RFC3339, endDate)
		contracts = append(contracts, c)
	}
	return contracts, nil
}

// This is used in the local GET /contracts and loadDbintoMemory
func (db *ProviderDB) GetAllContracts() ([]*core.Contract, error) {
	rows, err := db.Query(`SELECT ContractId, RenterId, ProviderId, StorageSpace, 
		RenterSignature, ProviderSignature, StorageFee, StartDate, EndDate FROM contracts`)
	if err != nil {
		return nil, err
	}
	var contracts []*core.Contract

	for rows.Next() {
		c := &core.Contract{}
		// scan does not parse these directly into time.Time correctly
		var startDate string
		var endDate string
		err = rows.Scan(&c.ID, &c.RenterId, &c.ProviderId, &c.StorageSpace, &c.RenterSignature, &c.ProviderSignature, &c.StorageFee, &startDate, &endDate)
		if err != nil {
			return nil, err
		}
		c.StartDate, _ = time.Parse(time.RFC3339, startDate)
		c.EndDate, _ = time.Parse(time.RFC3339, endDate)
		contracts = append(contracts, c)
	}
	return contracts, nil
}

func (db *ProviderDB) InsertBlock(renterId string, blockId string, size int64) error {
	stmt, err := db.Prepare(`INSERT INTO blocks (RenterId, BlockId, Size) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(renterId, blockId, size)
	if err != nil {
		return err
	}
	return nil
}

func (db *ProviderDB) DeleteBlockById(blockId string) error {
	stmt, err := db.Prepare(`DELETE from blocks where BlockId=?`)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(blockId)
	if err != nil {
		return err
	}
	return nil
}

// This is used in GET /renter-info
func (db *ProviderDB) GetBlocksByRenter(renterId string) ([]*BlockInfo, error) {
	query := fmt.Sprintf(`SELECT BlockId, Size FROM blocks where RenterId='%s'`, renterId)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	var blocks []*BlockInfo

	for rows.Next() {
		b := &BlockInfo{}
		err = rows.Scan(&b.BlockId, &b.Size)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, nil
}

// This is only used in LoadDbintoMemory
func (db *ProviderDB) GetAllBlocks() ([]*BlockInfo, error) {
	rows, err := db.Query(`SELECT RenterId, BlockId, Size FROM blocks`)
	if err != nil {
		return nil, err
	}
	var blocks []*BlockInfo

	for rows.Next() {
		b := &BlockInfo{}
		err = rows.Scan(&b.RenterId, &b.BlockId, &b.Size)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, nil
}

//  Loads basic memory objects from db
//  These will be recalculated based on db state at each restart
//  (potentially useful for maintenance also)
// - provider.StorageReserved
// - provider.StorageUsed
// - provider.TotalBlocks
// - provider.TotalContracts
// - provider.renters {
// 	   - StorageUsed
//     - StorageReserved
//   }
func (p *Provider) LoadDBIntoMemory() error {
	p.StorageReserved = 0
	p.StorageUsed = 0
	p.TotalBlocks = 0
	p.TotalContracts = 0
	p.renters = make(map[string]*RenterInfo, 0)

	contracts, err := p.db.GetAllContracts()
	if err != nil {
		// fatal?
		return err
	}
	for _, c := range contracts {
		_, ok := p.renters[c.RenterId]
		if !ok {
			p.renters[c.RenterId] = &RenterInfo{}
		}
		p.renters[c.RenterId].StorageReserved += c.StorageSpace
		p.StorageReserved += c.StorageSpace
		p.TotalContracts++
	}
	blocks, err := p.db.GetAllBlocks()
	if err != nil {
		// fatal?
		return err
	}
	for _, b := range blocks {
		_, ok := p.renters[b.RenterId]
		if !ok {
			// TODO: block with no associated contract?
			return nil
		}
		p.renters[b.RenterId].StorageUsed += b.Size
		p.StorageUsed += b.Size
		p.TotalBlocks++
	}
	return nil
}

// Initializes an empty stats response
func makeStatsResp() *getStatsResp {
	var timestamps []string
	t := time.Now().Truncate(time.Hour)
	currTime := t.Add(-1 * time.Hour * 24)
	for currTime != t {
		currTime = currTime.Add(time.Hour)
		timestamps = append(timestamps, currTime.Format(time.RFC3339))
	}
	resp := &getStatsResp{
		ActivityCounter: &Activity{
			Timestamps:          timestamps,
			BlockUploads:        make([]int64, 24),
			BlockDownloads:      make([]int64, 24),
			BlockDeletions:      make([]int64, 24),
			BytesUploaded:       make([]int64, 24),
			BytesDownloaded:     make([]int64, 24),
			StorageReservations: make([]int64, 24),
		},
		RecentSummary: &Recents{
			Hour: &Summary{
				BlockUploads:        0,
				BlockDownloads:      0,
				BlockDeletions:      0,
				StorageReservations: 0,
			},
			Day: &Summary{
				BlockUploads:        0,
				BlockDownloads:      0,
				BlockDeletions:      0,
				StorageReservations: 0,
			},
			Week: &Summary{
				BlockUploads:        0,
				BlockDownloads:      0,
				BlockDeletions:      0,
				StorageReservations: 0,
			},
		},
	}
	return resp
}
