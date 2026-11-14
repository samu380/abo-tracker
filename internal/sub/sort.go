package sub

import "sort"

func sortVisible(subs []Subscription) {
	sort.SliceStable(subs, func(i, j int) bool {
		if subs[i].Status != subs[j].Status {
			return subs[i].Status == StatusActive
		}
		if !subs[i].NextBilling.Equal(subs[j].NextBilling) {
			return subs[i].NextBilling.Before(subs[j].NextBilling)
		}
		return subs[i].Name < subs[j].Name
	})
}
