package coordinator

import "github.com/evcc-io/evcc/api"

// Excluded reports vehicles that status guessing must never assign: load management's car
// detection places them by physical proof instead (core/supercharge/locate.go)
var Excluded func(api.Vehicle) bool
