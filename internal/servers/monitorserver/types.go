package monitorserver

// queryRequest is the Log Analytics query request body.
type queryRequest struct {
	Query    string `json:"query"`
	Timespan string `json:"timespan,omitempty"`
}

// queryResponse is the Log Analytics query response body.
type queryResponse struct {
	Tables []table `json:"tables"`
}

// table is one result table in a query response.
type table struct {
	Name    string   `json:"name"`
	Columns []column `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// column describes one column of a result table.
type column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
