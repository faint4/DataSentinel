package rules

import "github.com/datasentinel/datasentinel/model"

// Classify determines the security level based on all matches found.
func Classify(matches []model.Match) model.Level {
	if len(matches) == 0 {
		return model.L1Public
	}

	l3Count := 0
	for _, m := range matches {
		switch m.Category {
		case model.CatPrivateKey, model.CatAWSKey, model.CatGitHubToken, model.CatHighEntropy:
			return model.L5Restricted
		case model.CatIDCard, model.CatPhone, model.CatBankCard, model.CatUSCC:
			l3Count++
		}
	}

	if l3Count >= 3 {
		return model.L4Secret
	}
	if l3Count > 0 {
		return model.L3Confidential
	}
	return model.L2Internal
}
