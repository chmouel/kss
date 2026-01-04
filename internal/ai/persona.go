package ai

func personaDisplayName(persona string) string {
	personaDisplay := map[string]string{
		"neutral":  "Neutral",
		"butler":   "🤵 Alfred",
		"sergeant": "🪖 The Drill Sergeant",
		"hacker":   "⌨️ The Cyberpunk Hacker",
		"pirate":   "🏴‍☠️ The Pirate",
		"genz":     "✨ The Gen Z Influencer",
	}
	if displayName := personaDisplay[persona]; displayName != "" {
		return displayName
	}
	return persona
}

func personaInstructions(persona string) string {
	switch persona {
	case "neutral":
		return "Use a neutral, technical tone. No persona, no slang, no flourishes."
	case "sergeant":
		return "Speak in the persona of a stern Drill Sergeant. Be demanding and direct, but keep it professional. Use caps for emphasis."
	case "hacker":
		return "Speak in the persona of an edgy cyberpunk hacker. Use technical slang like 'glitch', 'patching the ghost', 'zero-day', and 'mainframe'. Be cool and efficient."
	case "pirate":
		return "Speak in the persona of a rough pirate. Use 'Arrgh', 'matey', and nautical terms. Be gritty but helpful."
	case "genz":
		return "Speak in the persona of a Gen Z influencer. Use 'no cap', 'it's giving', 'shook', and 'vibe check'. Use plenty of emojis."
	default:
		return "Speak in the persona of Alfred, a refined British butler. Be polite, formal, but efficient. Address the user as 'sir'. Never use the word 'master'."
	}
}
