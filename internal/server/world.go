package server

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

var roomIDPattern = regexp.MustCompile(`^loc\.[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)

var directions = map[string]bool{
	"north": true, "south": true, "east": true,
	"west": true, "up": true, "down": true,
}

// Room is the static part of a location. Runtime players are held by Server.
type Room struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Exits       map[string]string `json:"exits"`
}

type World struct {
	Start     string          `json:"start"`
	Locations map[string]Room `json:"locations"`
}

func LoadWorld(path string) (*World, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read world: %w", err)
	}
	var file struct {
		World World `json:"world"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse world: %w", err)
	}
	world := &file.World
	if world.Start == "" {
		world.Start = "loc.town_square"
	}
	if len(world.Locations) == 0 {
		return nil, fmt.Errorf("world has no locations")
	}
	if _, ok := world.Locations[world.Start]; !ok {
		return nil, fmt.Errorf("start room %q does not exist", world.Start)
	}
	for id, room := range world.Locations {
		if len(id) > 64 || !roomIDPattern.MatchString(id) {
			return nil, fmt.Errorf("invalid room ID %q", id)
		}
		if room.Name == "" || room.Description == "" {
			return nil, fmt.Errorf("room %q needs name and description", id)
		}
		for direction, target := range room.Exits {
			if !directions[direction] {
				return nil, fmt.Errorf("room %q has invalid direction %q", id, direction)
			}
			if _, ok := world.Locations[target]; !ok {
				return nil, fmt.Errorf("room %q exit %q references unknown room %q", id, direction, target)
			}
		}
		room.ID = id
		if room.Exits == nil {
			room.Exits = map[string]string{}
		}
		world.Locations[id] = room
	}
	return world, nil
}
