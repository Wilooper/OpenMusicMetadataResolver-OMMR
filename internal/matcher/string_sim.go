package matcher

import (
	"math"
	"strings"
)

// JaroWinkler computes Jaro-Winkler string similarity in range [0.0, 1.0].
func JaroWinkler(s1, s2 string) float64 {
	j := jaro(s1, s2)
	if j < 0.7 {
		return j
	}

	prefix := 0
	maxPrefix := 4
	for i := 0; i < len(s1) && i < len(s2) && i < maxPrefix; i++ {
		if s1[i] == s2[i] {
			prefix++
		} else {
			break
		}
	}

	return j + float64(prefix)*0.1*(1.0-j)
}

func jaro(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	l1, l2 := len(s1), len(s2)
	if l1 == 0 || l2 == 0 {
		return 0.0
	}

	matchDistance := int(math.Max(float64(l1), float64(l2))/2.0) - 1
	if matchDistance < 0 {
		matchDistance = 0
	}

	s1Matches := make([]bool, l1)
	s2Matches := make([]bool, l2)

	matches := 0
	transpositions := 0

	for i := 0; i < l1; i++ {
		start := int(math.Max(0, float64(i-matchDistance)))
		end := int(math.Min(float64(i+matchDistance+1), float64(l2)))

		for j := start; j < end; j++ {
			if s2Matches[j] {
				continue
			}
			if s1[i] != s2[j] {
				continue
			}
			s1Matches[i] = true
			s2Matches[j] = true
			matches++
			break
		}
	}

	if matches == 0 {
		return 0.0
	}

	k := 0
	for i := 0; i < l1; i++ {
		if !s1Matches[i] {
			continue
		}
		for !s2Matches[k] {
			k++
		}
		if s1[i] != s2[k] {
			transpositions++
		}
		k++
	}

	m := float64(matches)
	t := float64(transpositions) / 2.0

	return (m/float64(l1) + m/float64(l2) + (m-t)/m) / 3.0
}

// TokenSetRatio computes token intersection overlap between two strings in range [0.0, 1.0].
func TokenSetRatio(s1, s2 string) float64 {
	t1 := strings.Fields(strings.ToLower(s1))
	t2 := strings.Fields(strings.ToLower(s2))

	if len(t1) == 0 && len(t2) == 0 {
		return 1.0
	}
	if len(t1) == 0 || len(t2) == 0 {
		return 0.0
	}

	set1 := make(map[string]bool)
	for _, w := range t1 {
		set1[w] = true
	}

	set2 := make(map[string]bool)
	for _, w := range t2 {
		set2[w] = true
	}

	intersection := 0
	for w := range set1 {
		if set2[w] {
			intersection++
		}
	}

	union := len(set1)
	for w := range set2 {
		if !set1[w] {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}
