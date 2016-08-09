package history

import (
	"database/sql"
	"os"
)

// A history DB that uses a SQL database
type sqlHistoryDB struct {
	driver *sql.DB
}

func NewSQL(driver, datasource string) (DB, error) {
	db, err := sql.Open(driver, datasource)
	if err != nil {
		return nil, err
	}
	historyDB := &sqlHistoryDB{driver: db}
	return historyDB, historyDB.ensureTables()
}

func (db *sqlHistoryDB) queryEvents(query string, params ...interface{}) ([]Event, error) {
	eventRows, err := db.driver.Query(query, params...)

	if err != nil {
		return nil, err
	}
	defer eventRows.Close()

	events := []Event{}
	for eventRows.Next() {
		var event Event
		eventRows.Scan(&event.Service, &event.Msg, &event.Stamp)
		events = append(events, event)
	}

	if err = eventRows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (db *sqlHistoryDB) AllEvents(namespace string) ([]Event, error) {
	return db.queryEvents(`SELECT service, message, stamp
                           FROM history
                           WHERE namespace = $1
                           ORDER BY service, stamp DESC`, namespace)
}

func (db *sqlHistoryDB) EventsForService(namespace, service string) ([]Event, error) {
	return db.queryEvents(`SELECT service, message, stamp
                           FROM history
                           WHERE namespace = $1 AND service = $2
                           ORDER BY stamp DESC`, namespace, service)
}

func (db *sqlHistoryDB) LogEvent(namespace, service, msg string) error {
	tx, err := db.driver.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`INSERT INTO history
                       (namespace, service, message, stamp)
                       VALUES ($1, $2, $3, now())`, namespace, service, msg)
	if err == nil {
		err = tx.Commit()
	}
	return err
}

func (db *sqlHistoryDB) ensureTables() (err error) {
	// ql requires a temp directory, but will apparently not create it
	// if it doesn't exist; and that can be the case when run inside a
	// container.
	os.Mkdir(os.TempDir(), 0777)

	tx, err := db.driver.Begin()
	if err != nil {
		return err
	}
	// cznic/ql has its own idea of types; this will need to be
	// adapted for other DB drivers.
	// http://godoc.org/github.com/cznic/ql#hdr-Types
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS history
             (namespace string NOT NULL,
              service   string NOT NULL,
              message   string NOT NULL,
              stamp     time NOT NULL)`)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}
