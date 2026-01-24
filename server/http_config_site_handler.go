package server

import (
	"net/http"

	"github.com/evcc-io/evcc/core/site"
	"github.com/evcc-io/evcc/util/config"
)

// siteHandler returns a device configurations by class
func siteHandler(site site.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := struct {
			Title   string   `json:"title"`
			Grid    string   `json:"grid"`
			PV      []string `json:"pv"`
			Battery []string `json:"battery"`
			Aux     []string `json:"aux"`
			Ext     []string `json:"ext"`
		}{
			Title:   site.GetTitle(),
			Grid:    site.GetGridMeterRef(),
			PV:      site.GetPVMeterRefs(),
			Battery: site.GetBatteryMeterRefs(),
			Aux:     site.GetAuxMeterRefs(),
			Ext:     site.GetExtMeterRefs(),
		}

		jsonWrite(w, res)
	}
}

func validateRefs(w http.ResponseWriter, refs []string) bool {
	for _, m := range refs {
		if _, err := config.Meters().ByName(m); err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return false
		}
	}
	return true
}

// siteHandler returns a device configurations by class
func updateSiteHandler(site site.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Title   *string
			Grid    *string
			PV      *[]string
			Battery *[]string
			Aux     *[]string
			Ext     *[]string
		}

		if err := jsonDecoder(r.Body).Decode(&payload); err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}

		if payload.Title != nil {
			site.SetTitle(*payload.Title)
		}

		if payload.Grid != nil {
			if *payload.Grid != "" && !validateRefs(w, []string{*payload.Grid}) {
				return
			}

			site.SetGridMeterRef(*payload.Grid)
			setConfigDirty()
		}

		if payload.PV != nil {
			if !validateRefs(w, *payload.PV) {
				return
			}

			site.SetPVMeterRefs(*payload.PV)
			setConfigDirty()
		}

		if payload.Battery != nil {
			if !validateRefs(w, *payload.Battery) {
				return
			}

			site.SetBatteryMeterRefs(*payload.Battery)
			setConfigDirty()
		}

		if payload.Aux != nil {
			if !validateRefs(w, *payload.Aux) {
				return
			}

			site.SetAuxMeterRefs(*payload.Aux)
			setConfigDirty()
		}

		if payload.Ext != nil {
			if !validateRefs(w, *payload.Ext) {
				return
			}

			site.SetExtMeterRefs(*payload.Ext)
			setConfigDirty()
		}

		status := map[bool]int{false: http.StatusOK, true: http.StatusAccepted}
		w.WriteHeader(status[ConfigDirty()])
	}
}

// priorityHandler handles the priority updates
func priorityHandler(site site.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var loadpointIDs []int
		if err := jsonDecoder(r.Body).Decode(&loadpointIDs); err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}

		// The logic is to assign priority based on the index in the array.
		// Higher index -> Higher Priority? Or First = High Prio?
		// User said: "drag and drop... to order the priority".
		// Usually Top = High Priority.
		// So we should assign priorities in descending order.
		// ID[0] = Max Prio (len)
		// ID[last] = Min Prio (1)

		lps := site.Loadpoints()
		if len(loadpointIDs) != len(lps) {
			// This might be tricky if IDs don't match indices directly or if there's a mismatch count.
			// Let's assume the frontend sends a list of indices (0-based) or IDs (1-based)?
			// The UI typically knows IDs. `site.Loadpoints` returns a slice.
			// Loadpoint IDs in API are 1-based (index + 1).
		}

		// Map ID to Loadpoint
		// Assuming simple 1-based indexing for now as per `RegisterSiteHandlers` loop:
		// for id, lp := range site.Loadpoints() ... /loadpoints/%d (id+1)

		maxPrio := len(loadpointIDs)
		for i, id := range loadpointIDs {
			// Find LP with this ID
			// 1-based ID -> 0-based index
			idx := id - 1
			if idx >= 0 && idx < len(lps) {
				// Priority: Top of list = Highest Priority
				prio := maxPrio - i
				lps[idx].SetPriority(prio)
			}
		}

		jsonWrite(w, nil)
	}
}
