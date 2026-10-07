package server

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var roomIDPattern = regexp.MustCompile(`^loc\.[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)
var itemIDPattern = regexp.MustCompile(`^item\.[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)

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
	Items       []string          `json:"items,omitempty"`
}

// Each catalogue entry describes one physical instance, identified by its ID.
type Item struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Obtainable bool   `json:"obtainable"`
}

type World struct {
	Start     string          `json:"start"`
	Locations map[string]Room `json:"locations"`
	Items     map[string]Item `json:"-"`
}

func LoadWorld(path string) (*World, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read world: %w", err)
	}
	var file struct {
		World struct {
			Start     string          `json:"start"`
			Locations map[string]Room `json:"locations"`
			Items     []Item          `json:"items"`
		} `json:"world"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse world: %w", err)
	}
	world := &World{Start: file.World.Start, Locations: file.World.Locations, Items: make(map[string]Item)}
	for _, item := range file.World.Items {
		if len(item.ID) > 64 || !itemIDPattern.MatchString(item.ID) {
			return nil, fmt.Errorf("invalid item ID %q", item.ID)
		}
		if strings.TrimSpace(item.Name) == "" {
			return nil, fmt.Errorf("item %q needs a display name", item.ID)
		}
		if item.Name != strings.TrimSpace(item.Name) || strings.ContainsAny(item.Name, "\t\r\n") {
			return nil, fmt.Errorf("item %q display name must be a single line without tabs or surrounding spaces", item.ID)
		}
		if _, exists := world.Items[item.ID]; exists {
			return nil, fmt.Errorf("duplicate item ID %q", item.ID)
		}
		world.Items[item.ID] = item
	}
	if world.Start == "" {
		world.Start = "loc.town_square"
	}
	if len(world.Locations) == 0 {
		return nil, fmt.Errorf("world has no locations")
	}
	if _, ok := world.Locations[world.Start]; !ok {
		return nil, fmt.Errorf("start room %q does not exist", world.Start)
	}
	placements := make(map[string]string)
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
		for _, itemID := range room.Items {
			if _, exists := world.Items[itemID]; !exists {
				return nil, fmt.Errorf("room %q references unknown item %q", id, itemID)
			}
			if previous, exists := placements[itemID]; exists {
				return nil, fmt.Errorf("item %q placed more than once (rooms %q and %q)", itemID, previous, id)
			}
			placements[itemID] = id
		}
		room.ID = id
		if room.Exits == nil {
			room.Exits = map[string]string{}
		}
		world.Locations[id] = room
	}
	for id := range world.Items {
		if _, placed := placements[id]; !placed {
			return nil, fmt.Errorf("item %q has no initial room", id)
		}
	}
	return world, nil
}
