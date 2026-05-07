package app

import "embed"

//go:embed all:embedded/pets
var petAssets embed.FS
