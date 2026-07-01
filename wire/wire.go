// Package wire holds the WebSocket message DTOs exchanged between the Klutch
// backend hub and this print agent. It is the ONLY contract shared with the
// backend, never domain types, and is kept dependency-free (stdlib only) so the
// agent binary stays thin.
//
// This is a synced copy of the backend's internal/printing/wire package. The two
// must stay byte-compatible: this is a wire protocol, so change both sides
// together and bump the protocol deliberately.
package wire

import "encoding/json"

// Message type discriminators (the Envelope.Type values).
const (
	// Agent → server.
	TypeHello        = "hello"         // advertise printers on connect
	TypePrinters     = "printers"      // re-advertise the printer set (sent only when it changed)
	TypeJobResult    = "job_result"    // ACK a job printed / failed
	TypeStatusReport = "status_report" // a printer changed state

	// Server → agent.
	TypeJob = "job" // a print job to transport
)

// Envelope frames every message: a Type discriminator plus the raw payload.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Encode builds an Envelope around a typed payload.
func Encode(msgType string, payload any) (Envelope, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Type: msgType, Data: b}, nil
}

// Printer is one printer the agent advertises in Hello: just identity and a
// display description. The agent does NOT classify (the OS can't reliably tell a
// thermal printer from an inkjet); the operator tags kind/paper-width/color in
// the dashboard, and that tag is preserved across reconnects.
type Printer struct {
	Name        string `json:"name"`
	Description string `json:"description"` // make/model for display
}

// Hello is the first message after connect: every printer connected to the agent
// PC. Tenant and branch are NOT carried; the server derives them from the
// authenticated device token, never trusting the client.
//
// The same shape is reused for TypePrinters, which the agent re-sends ONLY when
// its enumerated printer set changes (a printer plugged/unplugged or its port
// changed); not on every poll. The server reconciles both identically.
type Hello struct {
	Printers []Printer `json:"printers"`
}

// Job is a print job pushed to the agent. Payload is base64 (the rendered PDF or
// ESC/POS raster bytes); the agent only transports it. PrinterName is the OS
// queue the agent must dispatch to (`lp -d <PrinterName>` / the Windows
// spooler): the same name the agent advertised in Hello and the backend stored,
// resolved server-side from the job's printer id so the agent never maps ids.
type Job struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"` // pdf | escpos_raster
	PrinterName string `json:"printer_name"`
	DocumentRef string `json:"document_ref"`
	PayloadB64  string `json:"payload_b64"`
}

// JobResult ACKs a job's outcome.
type JobResult struct {
	JobID string `json:"job_id"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// StatusReport reports a printer state change (e.g. ran out of paper).
type StatusReport struct {
	PrinterName string `json:"printer_name"`
	Status      string `json:"status"`
}
