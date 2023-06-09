// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package name

import (
	"math/rand"
	"strconv"
)

var (
	left = [...]string{
		"admiring",
		"adoring",
		"affectionate",
		"agitated",
		"amazing",
		"angry",
		"awesome",
		"beautiful",
		"blissful",
		"bold",
		"boring",
		"brave",
		"busy",
		"charming",
		"clever",
		"cool",
		"compassionate",
		"competent",
		"condescending",
		"confident",
		"cranky",
		"crazy",
		"dazzling",
		"determined",
		"distracted",
		"dreamy",
		"eager",
		"ecstatic",
		"elastic",
		"elated",
		"elegant",
		"eloquent",
		"epic",
		"exciting",
		"fervent",
		"festive",
		"flamboyant",
		"focused",
		"friendly",
		"frosty",
		"funny",
		"gallant",
		"gifted",
		"goofy",
		"gracious",
		"great",
		"happy",
		"hardcore",
		"heuristic",
		"hopeful",
		"hungry",
		"infallible",
		"inspiring",
		"intelligent",
		"interesting",
		"jolly",
		"jovial",
		"keen",
		"kind",
		"laughing",
		"loving",
		"lucid",
		"magical",
		"mystifying",
		"modest",
		"musing",
		"naughty",
		"nervous",
		"nice",
		"nifty",
		"nostalgic",
		"objective",
		"optimistic",
		"peaceful",
		"pedantic",
		"pensive",
		"practical",
		"priceless",
		"quirky",
		"quizzical",
		"recursing",
		"relaxed",
		"reverent",
		"romantic",
		"sad",
		"serene",
		"sharp",
		"silly",
		"sleepy",
		"stoic",
		"strange",
		"stupefied",
		"suspicious",
		"sweet",
		"tender",
		"thirsty",
		"trusting",
		"unruffled",
		"upbeat",
		"vibrant",
		"vigilant",
		"vigorous",
		"wizardly",
		"wonderful",
		"xenodochial",
		"youthful",
		"zealous",
		"zen",
	}

	// KraftKit, starting from 0.6.0, generates names from notable apes (gorillas,
	// monkeys, chimpanzee, orangutans, bonobos, baboons, etc.).  The system is
	// inspired from Docker's name generation system.  This list is non-exahustive
	// and retrieved by parsing Wikipedia for individuals.
	right = [...]string{
		// See https://en.wikipedia.org/wiki/Abang_(orangutan)
		"abang",
		// See https://en.wikipedia.org/wiki/Ah_Meng
		"ahmeng",
		// See https://en.wikipedia.org/wiki/Ai_(chimpanzee)
		"ai",
		// See https://en.wikipedia.org/wiki/Albert_II_(monkey)
		"albertii",
		// See https://en.wikipedia.org/wiki/Alfred_the_Gorilla
		"alfred",
		// See https://en.wikipedia.org/wiki/ANDi
		"andi",
		// See https://en.wikipedia.org/wiki/Ayumu_(chimpanzee)
		"ayumu",
		// See https://en.wikipedia.org/wiki/Azalea_(chimpanzee)
		"azalea",
		// See https://en.wikipedia.org/wiki/Azy_(orangutan)
		"azy",
		// See https://en.wikipedia.org/wiki/Babec
		"babec",
		// See https://en.wikipedia.org/wiki/Binti_Jua
		"bintijua",
		// See https://en.wikipedia.org/wiki/Bobo_(gorilla)
		"bobo",
		// See https://en.wikipedia.org/wiki/Bokito_(gorilla)
		"bokito",
		// See https://en.wikipedia.org/wiki/Bonnie_(orangutan)
		"bonnie",
		// See https://en.wikipedia.org/wiki/Bubbles_(chimpanzee)
		"bubbles",
		// See https://en.wikipedia.org/wiki/Chantek
		"chantek",
		// See https://en.wikipedia.org/wiki/Charles_the_Gorilla
		"charles",
		// See https://en.wikipedia.org/wiki/Colo_(gorilla)
		"colo",
		// See https://en.wikipedia.org/wiki/Congo_(chimpanzee)
		"congo",
		// See https://en.wikipedia.org/wiki/Crystal_the_Monkey
		"crystal",
		// See https://en.wikipedia.org/wiki/Darwin_(monkey)
		"darwin",
		// See https://en.wikipedia.org/wiki/David_Greybeard
		"davidgreybeard",
		// See https://en.wikipedia.org/wiki/Edgar_(chimpanzee)
		"edgar",
		// See https://en.wikipedia.org/wiki/Enos_(chimpanzee)
		"enos",
		// See https://en.wikipedia.org/wiki/Faben
		"faben",
		// See https://en.wikipedia.org/wiki/Fanni_(chimpanzee)
		"fanni",
		// See https://en.wikipedia.org/wiki/Fatou_(gorilla)
		"fatou",
		// See https://en.wikipedia.org/wiki/Faustino_(chimpanzee)
		"faustino",
		// See https://en.wikipedia.org/wiki/Ferdinand_(chimpanzee)
		"ferdinand",
		// See https://en.wikipedia.org/wiki/Fifi_(chimpanzee)
		"fifi",
		// See https://en.wikipedia.org/wiki/Figan
		"figan",
		// See https://en.wikipedia.org/wiki/Flame_(chimpanzee)
		"flame",
		// See https://en.wikipedia.org/wiki/Flint_(chimpanzee)
		"flint",
		// See https://en.wikipedia.org/wiki/Flirt_(chimpanzee)
		"flirt",
		// See https://en.wikipedia.org/wiki/Flo_(chimpanzee)
		"flo",
		// See https://en.wikipedia.org/wiki/Flossi
		"flossi",
		// See https://en.wikipedia.org/wiki/Fred_(baboon)
		"fred",
		// See https://en.wikipedia.org/wiki/Freud_(chimpanzee)
		"freud",
		// See https://en.wikipedia.org/wiki/Frodo_(chimpanzee)
		"frodo",
		// See https://en.wikipedia.org/wiki/Gaia_(chimpanzee)
		"gaia",
		// See https://en.wikipedia.org/wiki/Gargantua_(gorilla)
		"gargantua",
		// See https://en.wikipedia.org/wiki/Glitter_(chimpanzee)
		"glitter",
		// See https://en.wikipedia.org/wiki/Goblin_(chimpanzee)
		"goblin",
		// See https://en.wikipedia.org/wiki/Golden_(chimpanzee)
		"golden",
		// See https://en.wikipedia.org/wiki/Goliath_(chimpanzee)
		"goliath",
		// See https://en.wikipedia.org/wiki/Gordo_(monkey)
		"gordo",
		// See https://en.wikipedia.org/wiki/Gregoire_(chimpanzee)
		"gregoire",
		// See https://en.wikipedia.org/wiki/Gremlin_(chimpanzee)
		"gremlin",
		// See https://en.wikipedia.org/wiki/Gua_(chimpanzee)
		"gua",
		// See https://en.wikipedia.org/wiki/Guy_the_Gorilla
		"guy",
		// See https://en.wikipedia.org/wiki/Ham_(chimpanzee)
		"ham",
		// See https://en.wikipedia.org/wiki/Harambe
		"harambe",
		// See https://en.wikipedia.org/wiki/Zhong_Zhong_and_Hua_Hua
		"hua",
		// See https://en.wikipedia.org/wiki/Humphrey_(chimpanzee)
		"humphrey",
		// See https://en.wikipedia.org/wiki/Ivan_(gorilla)
		"ivan",
		// See https://en.wikipedia.org/wiki/Jacco_Macacco
		"jaccomacacco",
		// See https://en.wikipedia.org/wiki/Jack_(baboon)
		"jack",
		// See https://en.wikipedia.org/wiki/Corporal_Jackie
		"jackie",
		// See https://en.wikipedia.org/wiki/Jambo
		"jambo",
		// See https://en.wikipedia.org/wiki/Jenny_(gorilla)
		"jenny",
		// See https://en.wikipedia.org/wiki/Jenny_(orangutan)
		"jenny",
		// See https://en.wikipedia.org/wiki/Jiggs_(orangutan)
		"jiggs",
		// See https://en.wikipedia.org/wiki/Jinx_(chimpanzee)
		"jinx",
		// See https://en.wikipedia.org/wiki/Joe_Martin_(orangutan)
		"joemartin",
		// See https://en.wikipedia.org/wiki/John_Daniel_(gorilla)
		"johndaniel",
		// See https://en.wikipedia.org/wiki/John_Daniel_II_(gorilla)
		"johndanielii",
		// See https://en.wikipedia.org/wiki/Julius_(chimpanzee)
		"julius",
		// See https://en.wikipedia.org/wiki/Jumoke
		"jumoke",
		// See https://en.wikipedia.org/wiki/Kanzi
		"kanzi",
		// See https://en.wikipedia.org/wiki/Karen_(orangutan)
		"karen",
		// See https://en.wikipedia.org/wiki/Karta_(orangutan)
		"karta",
		// See https://en.wikipedia.org/wiki/Ken_Allen
		"kenallen",
		// See https://en.wikipedia.org/wiki/Koko_(gorilla)
		"koko",
		// See https://en.wikipedia.org/wiki/Kokomo_(gorilla)
		"kokomo",
		// See https://en.wikipedia.org/wiki/Kris_(chimpanzee)
		"kris",
		// See https://en.wikipedia.org/wiki/Lana_(chimpanzee)
		"lana",
		// See https://en.wikipedia.org/wiki/Little_Mama
		"littlemama",
		// See https://en.wikipedia.org/wiki/Loon_(monkey)
		"loon",
		// See https://en.wikipedia.org/wiki/Louis_(gorilla)
		"louis",
		// See https://en.wikipedia.org/wiki/Loulis
		"loulis",
		// See https://en.wikipedia.org/wiki/Lucy_(chimpanzee)
		"lucy",
		// See https://en.wikipedia.org/wiki/Macaco_Tião
		"macacotiao",
		// See https://en.wikipedia.org/wiki/Maggie_the_Monkey
		"maggie",
		// See https://en.wikipedia.org/wiki/Manis_(orangutan)
		"manis",
		// See https://en.wikipedia.org/wiki/Massa_(gorilla)
		"massa",
		// See https://en.wikipedia.org/wiki/Max_(gorilla)
		"max",
		// See https://en.wikipedia.org/wiki/Melissa_(chimpanzee)
		"melissa",
		// See https://en.wikipedia.org/wiki/Michael_(gorilla)
		"michael",
		// See https://en.wikipedia.org/wiki/Mike_(chimpanzee)
		"mike",
		// See https://en.wikipedia.org/wiki/Miss_Baker
		"missbaker",
		// See https://en.wikipedia.org/wiki/Moe_(chimpanzee)
		"moe",
		// See https://en.wikipedia.org/wiki/Moja_(chimpanzee)
		"moja",
		// See https://en.wikipedia.org/wiki/Natasha_(monkey)
		"natasha",
		// See https://en.wikipedia.org/wiki/Ndakasi
		"ndakasi",
		// See https://en.wikipedia.org/wiki/Ndume
		"ndume",
		// See https://en.wikipedia.org/wiki/Nico_(gorilla)
		"nico",
		// See https://en.wikipedia.org/wiki/Nim_Chimpsky
		"nimchimpsky",
		// See https://en.wikipedia.org/wiki/Nonja_(Austrian_orangutan) or
		// https://en.wikipedia.org/wiki/Nonja_(Malaysian_orangutan)
		"nonja",
		// See https://en.wikipedia.org/wiki/Nyota_(bonobo)
		"nyota",
		// See https://en.wikipedia.org/wiki/Oliver_(chimpanzee)
		"oliver",
		// See https://en.wikipedia.org/wiki/Ozzie_(gorilla)
		"ozzie",
		// See https://en.wikipedia.org/wiki/Panbanisha
		"panbanisha",
		// See https://en.wikipedia.org/wiki/Pankun
		"pankun",
		// See https://en.wikipedia.org/wiki/Panpanzee
		"panpanzee",
		// See https://en.wikipedia.org/wiki/Pattycake_(gorilla)
		"pattycake",
		// See https://en.wikipedia.org/wiki/Pierre_Brassau
		"pierrebrassau",
		// See https://en.wikipedia.org/wiki/Pockets_Warhol
		"pocketswarhol",
		// See https://en.wikipedia.org/wiki/Pogo_(gorilla)
		"pogo",
		// See https://en.wikipedia.org/wiki/Ramu_(monkey)
		"ramu",
		// See https://en.wikipedia.org/wiki/Rancho_(monkey)
		"rancho",
		// See https://en.wikipedia.org/wiki/Sami_(chimpanzee)
		"sami",
		// See https://en.wikipedia.org/wiki/Samson_(gorilla)
		"samson",
		// See https://en.wikipedia.org/wiki/Sandra_(orangutan)
		"sandra",
		// See https://en.wikipedia.org/wiki/Santino_(chimpanzee)
		"santino",
		// See https://en.wikipedia.org/wiki/Sarah_(chimpanzee)
		"sarah",
		// See https://en.wikipedia.org/wiki/Shabani_(gorilla)
		"shabani",
		// See https://en.wikipedia.org/wiki/Sheldon_(chimpanzee)
		"sheldon",
		// See https://en.wikipedia.org/wiki/Snowflake_(gorilla)
		"snowflake",
		// See https://en.wikipedia.org/wiki/Sultan_(chimpanzee)
		"sultan",
		// See https://en.wikipedia.org/wiki/Tetra_(monkey)
		"tetra",
		// See https://en.wikipedia.org/wiki/Timmy_(gorilla)
		"timmy",
		// See https://en.wikipedia.org/wiki/Titan_(chimpanzee)
		"titan",
		// See https://en.wikipedia.org/wiki/Titus_(gorilla)
		"titus",
		// See https://en.wikipedia.org/wiki/Tonda_(orangutan)
		"tonda",
		// See https://en.wikipedia.org/wiki/Toto_(gorilla)
		"toto",
		// See https://en.wikipedia.org/wiki/Travis_(chimpanzee)
		"travis",
		// See https://en.wikipedia.org/wiki/Trudy_(gorilla)
		"trudy",
		// See https://en.wikipedia.org/wiki/Twelves
		"twelves",
		// See https://en.wikipedia.org/wiki/Viki_(chimpanzee)
		"viki",
		// See https://en.wikipedia.org/wiki/Washoe_(chimpanzee)
		"washoe",
		// See https://en.wikipedia.org/wiki/Whiplash_the_Cowboy_Monkey
		"whiplash",
		// See https://en.wikipedia.org/wiki/Wilkie_(chimpanzee)
		"wilkie",
		// See https://en.wikipedia.org/wiki/Willie_B.
		"willieb",
		// See https://en.wikipedia.org/wiki/Zhong_Zhong_and_Hua_Hua
		"zhong",
	}
)

// GetRandomName generates a random name from the list of adjectives and surnames in this package
// formatted as "adjective_surname". For example 'focused_turing'. If retry is non-zero, a random
// integer between 0 and 10 will be added to the end of the name, e.g `focused_turing3`
func NewRandomMachineName(retry int) string {
	name := left[rand.Intn(len(left))] + "_" + right[rand.Intn(len(right))] //nolint:gosec // G404: Use of weak random number generator (math/rand instead of crypto/rand)

	if retry > 0 {
		name += strconv.Itoa(rand.Intn(10)) //nolint:gosec // G404: Use of weak random number generator (math/rand instead of crypto/rand)
	}
	return name
}
