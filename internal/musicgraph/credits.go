package musicgraph

type Contributor struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Provider string `json:"provider,omitempty"`
	NativeID string `json:"native_id,omitempty"`
	Local    bool   `json:"local,omitempty"`
}

type CreditDetails struct {
	Contributors []Contributor `json:"contributors"`
	Credits      []Edge        `json:"credits"`
	Truncated    bool          `json:"truncated,omitempty"`
}

// Credit edges refer to graph subjects, preserving recording/work/release scope.
func DetailedCredits(g Graph) *CreditDetails {
	result := &CreditDetails{Truncated: g.Truncated}
	used := map[string]bool{}
	for _, e := range g.Edges {
		if e.Type != "credit" {
			continue
		}
		result.Credits = append(result.Credits, e)
		used[e.From] = true
	}
	if len(result.Credits) == 0 {
		return nil
	}
	for _, n := range g.Nodes {
		if used[n.ID] {
			result.Contributors = append(result.Contributors, Contributor{ID: n.ID, Name: n.Name, Provider: n.Provider, NativeID: n.NativeID, Local: n.Local})
		}
	}
	return result
}

func IsCreditRole(role string) bool { return creditRole(role) }
