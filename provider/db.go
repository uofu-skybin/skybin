package provider

import (
	"database/sql"
	"fmt"
	"skybin/core"
	"time"
)

func (p *Provider) setup_db(path string) (*sql.DB, error) {
	db, _ := sql.Open("sqlite3", path)
	// Create contracts table
	stmt, err := db.Prepare("CREATE TABLE IF NOT EXISTS contracts (id INTEGER PRIMARY KEY, ContractId TEXT, RenterId TEXT, ProviderId TEXT, StorageSpace INTEGER, StartDate TEXT, EndDate TEXT, RenterSignature TEXT, ProviderSignature TEXT)")
	if err != nil {
		return nil, err
	}
	stmt.Exec()

	// Create blocks table
	stmt, err = db.Prepare("CREATE TABLE IF NOT EXISTS blocks (id INTEGER PRIMARY KEY, RenterId TEXT, BlockId TEXT, Size INTEGER)")
	if err != nil {
		return nil, err
	}
	stmt.Exec()

	// Create renters table
	// TODO: might not need the storage used or reserved fields
	stmt, err = db.Prepare("CREATE TABLE IF NOT EXISTS renters (id INTEGER PRIMARY KEY, RenterId TEXT, StorageReserved INTEGER, StorageUsed INTEGER)")
	if err != nil {
		return nil, err
	}
	stmt.Exec()

	// Create activity table
	stmt, err = db.Prepare("CREATE TABLE IF NOT EXISTS activity ( `id` INTEGER PRIMARY KEY, `Period` TEXT, `Timestamp` TEXT, `BlockUploads` INTEGER DEFAULT 0, `BlockDownloads` INTEGER DEFAULT 0, `BlockDeletions` INTEGER DEFAULT 0, `BytesUploaded` INTEGER DEFAULT 0, `BytesDownloaded` INTEGER DEFAULT 0, `StorageReservations` INTEGER DEFAULT 0 )")
	if err != nil {
		return nil, err
	}
	stmt.Exec()

	return db, err
}

func (p *Provider) InsertBlock(renterId string, blockId string, size int64) error {
	stmt, err := p.db.Prepare("INSERT INTO blocks (RenterId, BlockId, Size) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(renterId, blockId, size)
	if err != nil {
		return err
	}
	return nil
}

// Insert new activity if activity doesn't exist on that interval
func (p *Provider) InsertActivity(period string, timestamp time.Time) error {
	stmt, err := p.db.Prepare("INSERT INTO activity (Period, Timestamp) Select ?, ? WHERE NOT EXISTS(SELECT 1 FROM activity WHERE Period = ? and Timestamp = ? )")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(period, timestamp.Format(time.RFC3339), period, timestamp.Format(time.RFC3339))
	if err != nil {
		return err
	}
	return nil
}

// Increment activity corresponding to interval by "value"
func (p *Provider) UpdateActivity(period string, timestamp time.Time, op string, value int64) error {
	query := fmt.Sprintf("UPDATE activity SET %s = %s + ? WHERE Timestamp = ? and Period = ?", op, op)

	stmt, err := p.db.Prepare(query)
	_, err = stmt.Exec(value, timestamp.Format(time.RFC3339), period)
	if err != nil {
		return err
	}
	return nil
}

// This is called by the local provider server on GET /stats
func (p *Provider) GetStatsResp() (*getStatsResp, error) {
	query := fmt.Sprintf("SELECT Period, Timestamp, BlockUploads, BlockDownloads, BlockDeletions, BytesUploaded, BytesDownloaded, StorageReservations FROM activity") // WHERE Period=%s", period)
	resp := p.makeStatsResp()
	rows, err := p.db.Query(query)
	if err != nil {
		return nil, err
	}
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
		rows.Scan(&period, &timestamp, &blockUploads, &blockDownloads, &blockDeletions, &bytesUploaded, &bytesDownloaded, &storageReservations)
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
		}
		if period == "hour" {
			resp.RecentSummary.Hour.BlockUploads += blockUploads
			resp.RecentSummary.Hour.BlockDownloads += blockDownloads
			resp.RecentSummary.Hour.BlockDeletions += blockDeletions
			resp.RecentSummary.Hour.StorageReservations += storageReservations
		}
		if period == "week" {
			resp.RecentSummary.Week.BlockUploads += blockUploads
			resp.RecentSummary.Week.BlockDownloads += blockDownloads
			resp.RecentSummary.Week.BlockDeletions += blockDeletions
			resp.RecentSummary.Week.StorageReservations += storageReservations
		}

	}
	return resp, nil
}

// Drop activity that is no longer in the scope of the interval
func (p *Provider) DeleteActivity() error {
	stmt, err := p.db.Prepare("DELETE from activity WHERE Period='hour' and Timestamp < ?")
	t := time.Now().Add(-1 * time.Hour)
	_, err = stmt.Exec(t.Format(time.RFC3339))
	if err != nil {
		return err
	}

	stmt, err = p.db.Prepare("DELETE from activity WHERE Period='day' and Timestamp < ?")
	t = time.Now().Add(-1 * time.Hour * 24)
	_, err = stmt.Exec(t.Format(time.RFC3339))
	if err != nil {
		return err
	}

	stmt, err = p.db.Prepare("DELETE from activity WHERE Period='week' and Timestamp < ?")
	t = time.Now().Add(-1 * time.Hour * 24 * 7)
	_, err = stmt.Exec(t.Format(time.RFC3339))
	if err != nil {
		return err
	}
	return nil
}

func (p *Provider) DeleteBlockById(blockId string) error {
	stmt, err := p.db.Prepare("DELETE from blocks where BlockId=?")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(blockId)
	if err != nil {
		return err
	}
	return nil
}

func (p *Provider) InsertContract(contract *core.Contract) error {
	stmt, err := p.db.Prepare("INSERT INTO contracts (ContractId, RenterId, ProviderId, StorageSpace, StartDate, EndDate, RenterSignature, ProviderSignature) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
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
	)
	return nil
}

func (p *Provider) GetContractsByRenter(renterId string) ([]*core.Contract, error) {
	query := fmt.Sprintf("SELECT ContractId, RenterId, ProviderId, StorageSpace, RenterSignature, ProviderSignature, StartDate, EndDate FROM contracts where RenterId='%s'", renterId)
	rows, err := p.db.Query(query)
	if err != nil {
		return nil, err
	}
	var contracts []*core.Contract

	for rows.Next() {
		c := &core.Contract{}
		// scan does not parse these directly into time.Time correctly
		var startDate string
		var endDate string
		rows.Scan(&c.ID, &c.RenterId, &c.ProviderId, &c.StorageSpace, &c.RenterSignature, &c.ProviderSignature, &startDate, &endDate)
		c.StartDate, _ = time.Parse(time.RFC3339, startDate)
		c.EndDate, _ = time.Parse(time.RFC3339, endDate)
		contracts = append(contracts, c)
	}
	return contracts, nil
}

func (p *Provider) GetBlocksByRenter(renterId string) ([]*BlockInfo, error) {
	query := fmt.Sprintf("SELECT BlockId, Size FROM blocks where RenterId='%s'", renterId)
	rows, err := p.db.Query(query)
	if err != nil {
		return nil, err
	}
	var blocks []*BlockInfo

	for rows.Next() {
		b := &BlockInfo{}
		rows.Scan(&b.BlockId, &b.Size)
		blocks = append(blocks, b)
	}
	return blocks, nil
}

func (p *Provider) GetAllBlocks() ([]*BlockInfo, error) {
	// query := fmt.Sprintf("SELECT BlockId, Size FROM blocks")
	rows, err := p.db.Query("SELECT RenterId, BlockId, Size FROM blocks")
	if err != nil {
		return nil, err
	}
	var blocks []*BlockInfo

	for rows.Next() {
		b := &BlockInfo{}
		rows.Scan(&b.RenterId, &b.BlockId, &b.Size)
		fmt.Println(b)
		blocks = append(blocks, b)
	}
	return blocks, nil
}

func (p *Provider) GetAllContracts() ([]*core.Contract, error) {
	rows, err := p.db.Query("SELECT ContractId, RenterId, ProviderId, StorageSpace, RenterSignature, ProviderSignature, StartDate, EndDate FROM contracts")
	if err != nil {
		return nil, err
	}
	var contracts []*core.Contract

	for rows.Next() {
		c := &core.Contract{}
		// scan does not parse these directly into time.Time correctly
		var startDate string
		var endDate string
		rows.Scan(&c.ID, &c.RenterId, &c.ProviderId, &c.StorageSpace, &c.RenterSignature, &c.ProviderSignature, &startDate, &endDate)
		c.StartDate, _ = time.Parse(time.RFC3339, startDate)
		c.EndDate, _ = time.Parse(time.RFC3339, endDate)
		contracts = append(contracts, c)
	}
	return contracts, nil
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

	contracts, err := p.GetAllContracts()
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
	blocks, err := p.GetAllBlocks()
	if err != nil {
		// fatal?
		return err
	}
	for _, b := range blocks {
		_, ok := p.renters[b.RenterId]
		if !ok {
			return nil
		}
		p.renters[b.RenterId].StorageUsed += b.Size
		p.StorageUsed += b.Size
		p.TotalBlocks++
	}
	return nil
}
