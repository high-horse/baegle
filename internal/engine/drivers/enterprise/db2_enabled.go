//go:build db2 && cgo

package enterprise

import _ "github.com/ibmdb/go_ibm_db"

const db2Enabled = true
