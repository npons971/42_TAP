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
var questIDPattern = regexp.MustCompile(`^quest\.[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)
var npcIDPattern = regexp.MustCompile(`^npc\.[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)

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
	NPCs        []string          `json:"npcs,omitempty"`
}

// Each catalogue entry describes one physical instance, identified by its ID.
type Item struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Obtainable      bool             `json:"obtainable"`
	Description     string           `json:"description,omitempty"`
	Type            string           `json:"type,omitempty"`
	HealValue       int              `json:"heal_value,omitempty"`
	DamageBonus     int              `json:"damage_bonus,omitempty"`
	DefenseBonus    int              `json:"defense_bonus,omitempty"`
	InitialLocation *InitialLocation `json:"initial_location,omitempty"`
}

// Room placements remain in Room.Items; only off-map reserves use this field.
type InitialLocation struct {
	Kind string `json:"kind"`
}

type WorldMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type World struct {
	Start    string        `json:"start"`
	Respawn  string        `json:"respawn"`
	Metadata WorldMetadata `json:"metadata"`
	// Raw definitions are retained; Rewards holds validated delivery/reward fields.
	Quests    map[string]json.RawMessage  `json:"quests"`
	Rewards   map[string]RewardDefinition `json:"-"`
	Locations map[string]Room             `json:"locations"`
	Items     map[string]Item             `json:"-"`
	NPCs      map[string]NPC              `json:"-"`
}

func LoadWorld(path string) (*World, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read world: %w", err)
	}
	var file struct {
		World struct {
			Start     string                     `json:"start"`
			Respawn   string                     `json:"respawn"`
			Metadata  WorldMetadata              `json:"metadata"`
			Quests    map[string]json.RawMessage `json:"quests"`
			Locations map[string]Room            `json:"locations"`
			Items     []Item                     `json:"items"`
			NPCs      []NPC                      `json:"npcs"`
		} `json:"world"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse world: %w", err)
	}
	world := &World{Start: file.World.Start, Locations: file.World.Locations, Items: make(map[string]Item)}
	world.Respawn, world.Metadata, world.Quests = file.World.Respawn, file.World.Metadata, file.World.Quests
	world.NPCs = make(map[string]NPC)
	for _, npc := range file.World.NPCs {
		if len(npc.ID) > 64 || !npcIDPattern.MatchString(npc.ID) {
			return nil, fmt.Errorf("invalid NPC ID %q", npc.ID)
		}
		if strings.TrimSpace(npc.Name) == "" || npc.Name != strings.TrimSpace(npc.Name) || strings.ContainsAny(npc.Name, "\t\r\n") {
			return nil, fmt.Errorf("NPC %q needs a single-line display name without tabs or surrounding spaces", npc.ID)
		}
		if npc.Role != "dialogue" && npc.Role != "quest_giver" && npc.Role != "enemy" {
			return nil, fmt.Errorf("NPC %q role must be dialogue, quest_giver or enemy", npc.ID)
		}
		if strings.TrimSpace(npc.Dialogue) == "" {
			return nil, fmt.Errorf("NPC %q needs dialogue", npc.ID)
		}
		if npc.Role == "enemy" && (npc.HP <= 0 || npc.AttackPower <= 0 || !npc.Hostile) {
			return nil, fmt.Errorf("enemy %q needs positive HP, attack power and hostile=true", npc.ID)
		}
		if npc.HP < 0 || npc.MaxHP < 0 || npc.AttackPower < 0 || npc.HP > npc.MaxHP {
			return nil, fmt.Errorf("NPC %q has invalid HP or attack power", npc.ID)
		}
		if _, exists := world.NPCs[npc.ID]; exists {
			return nil, fmt.Errorf("duplicate NPC ID %q", npc.ID)
		}
		world.NPCs[npc.ID] = npc
	}
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
		if item.InitialLocation != nil && item.InitialLocation.Kind != "reserve" {
			return nil, fmt.Errorf("item %q initial_location kind must be reserve; place room items in the room items list", item.ID)
		}
		if item.Type == "consumable" && (!item.Obtainable || item.HealValue <= 0) {
			return nil, fmt.Errorf("consumable %q must be obtainable and have positive healing", item.ID)
		}
		if item.HealValue < 0 || item.DamageBonus < 0 || item.DefenseBonus < 0 {
			return nil, fmt.Errorf("item %q bonuses and healing must be nonnegative", item.ID)
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
	if world.Respawn == "" {
		world.Respawn = world.Start
	}
	if _, ok := world.Locations[world.Respawn]; !ok {
		return nil, fmt.Errorf("respawn room %q does not exist", world.Respawn)
	}
	placements := make(map[string]string)
	npcPlacements := make(map[string]string)
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
			if world.Items[itemID].InitialLocation != nil {
				return nil, fmt.Errorf("item %q cannot be reserved and placed in a room", itemID)
			}
			if previous, exists := placements[itemID]; exists {
				return nil, fmt.Errorf("item %q placed more than once (rooms %q and %q)", itemID, previous, id)
			}
			placements[itemID] = id
		}
		for _, npcID := range room.NPCs {
			if _, exists := world.NPCs[npcID]; !exists {
				return nil, fmt.Errorf("room %q references unknown NPC %q", id, npcID)
			}
			if previous, exists := npcPlacements[npcID]; exists {
				return nil, fmt.Errorf("NPC %q placed more than once (rooms %q and %q)", npcID, previous, id)
			}
			npcPlacements[npcID] = id
		}
		room.ID = id
		if room.Exits == nil {
			room.Exits = map[string]string{}
		}
		world.Locations[id] = room
	}
	for id := range world.Items {
		if _, placed := placements[id]; !placed && world.Items[id].InitialLocation == nil {
			return nil, fmt.Errorf("item %q has no initial room", id)
		}
	}
	for id := range world.NPCs {
		if _, placed := npcPlacements[id]; !placed {
			return nil, fmt.Errorf("NPC %q has no initial room", id)
		}
	}
	if err := world.validateRewards(); err != nil {
		return nil, err
	}
	return world, nil
}
