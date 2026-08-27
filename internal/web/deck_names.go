package web

import (
	"fmt"
	"math/rand"
)

const (
	generatedDeckNameMaxLength = 32
	deckNameNumberRange        = 10_000
	deckNameStyleCount         = 4
)

var deckNameAdjectives = [...]string{
	"Shadow",
	"Neon",
	"Void",
	"Furious",
	"Arcane",
	"Bouncy",
	"Brave",
	"Caffeinated",
	"Chaotic",
	"Cosmic",
	"Cranky",
	"Crispy",
	"Cuddly",
	"Dapper",
	"Dizzy",
	"Electric",
	"Feral",
	"Fluffy",
	"Galactic",
	"Ghostly",
	"Glorious",
	"Grumpy",
	"Hasty",
	"Invisible",
	"Jolly",
	"Lucky",
	"Lunar",
	"Mighty",
	"Mystic",
	"Nervous",
	"Nimble",
	"Noisy",
	"Peculiar",
	"Pixelated",
	"Polite",
	"Prickly",
	"Salty",
	"Shiny",
	"Sleepy",
	"Sneaky",
	"Spicy",
	"Stealthy",
	"Stormy",
	"Tactical",
	"Tiny",
	"Toasty",
	"Turbo",
	"Unlikely",
	"Wobbly",
	"Unstoppable",
}

var deckNameNouns = [...]string{
	"Weaver",
	"Phantom",
	"Walker",
	"Tiger",
	"Panda",
	"Wolf",
	"Ninja",
	"Wizard",
	"Goblin",
	"Gremlin",
	"Badger",
	"Raccoon",
	"Pigeon",
	"Potato",
	"Toaster",
	"Noodle",
	"Turnip",
	"Hamster",
	"Goose",
	"Wombat",
	"Narwhal",
	"Capybara",
	"Axolotl",
	"Possum",
	"Ferret",
	"Gecko",
	"Llama",
	"Alpaca",
	"Penguin",
	"Puffin",
	"Otter",
	"Moose",
	"Moth",
	"Beetle",
	"Sprout",
	"Pickle",
	"Waffle",
	"Pancake",
	"Pretzel",
	"Burrito",
	"Meatball",
	"Dumpling",
	"Cupcake",
	"Teacup",
	"Slipper",
	"Keyboard",
	"SideQuest",
	"LootGoblin",
	"ManaGremlin",
	"Cardboard",
	"Backpack",
	"TaterTot",
	"DustBunny",
	"TrashPanda",
	"SnackBandit",
	"DiceGoblin",
	"MoonChicken",
	"CaveWizard",
	"SofaKnight",
	"CouchPotato",
}

// randomDeckName creates a disposable default that is intentionally more
// memorable than "Untitled Deck". Deck names are not identifiers, so this
// favors variety over database-backed uniqueness.
func randomDeckName() string {
	return generateDeckName(rand.Intn)
}

// generateDeckName accepts Intn as a dependency so tests can select every
// component deterministically without changing production randomness.
func generateDeckName(intn func(int) int) string {
	style := intn(deckNameStyleCount)
	adjective := deckNameAdjectives[intn(len(deckNameAdjectives))]
	noun := deckNameNouns[intn(len(deckNameNouns))]
	number := fmt.Sprintf("%02d", intn(deckNameNumberRange))

	switch style {
	case 1:
		return "xX_" + adjective + noun + number + "_Xx"
	case 2:
		return adjective + "_" + noun + "_" + number
	case 3:
		return noun + adjective + number
	default:
		return adjective + noun + number
	}
}
