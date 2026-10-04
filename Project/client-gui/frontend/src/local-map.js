// LOOK gives a local topology, not geographic coordinates. Keep every directed
// exit usable, including vertical/custom directions and two routes to one room.
const slots = {
  north: [450, 60], northeast: [710, 78], east: [760, 200],
  southeast: [710, 330], south: [450, 340], southwest: [190, 330],
  west: [140, 200], northwest: [190, 78], up: [710, 78], down: [190, 330],
};
const fallbackSlots = [[450, 60], [760, 200], [450, 340], [140, 200], [710, 78], [190, 330], [710, 330], [190, 78]];

export function buildLocalMap(room, names = {}) {
  const exits = Object.entries(room.exits || {});
  const occupied = new Set();
  // Reserve compass positions before placing custom directions.
  const positions = new Map();
  for (const [direction] of exits) {
    const position = slots[direction.toLowerCase()];
    if (position && !occupied.has(position.join(','))) {
      positions.set(direction, position);
      occupied.add(position.join(','));
    }
  }
  return exits.map(([direction, destination], index) => {
    let position = positions.get(direction);
    if (exits.length > 8) {
      const angle = -Math.PI / 2 + index * 2 * Math.PI / exits.length;
      position = [450 + 320 * Math.cos(angle), 200 + 145 * Math.sin(angle)];
    } else if (!position) {
      position = fallbackSlots.find(slot => !occupied.has(slot.join(',')));
      occupied.add(position.join(','));
    }
    const visited = Object.hasOwn(names, destination);
    return {
      direction, destination, x: position[0], y: position[1],
      label: (visited && names[destination]) || String(destination).replace(/^loc\./, '').replace(/_/g, ' '),
      visited,
    };
  });
}

export function buildLocalMarkers(room) {
  const markers = [
    ...(room.npcs || []).map(entity => ({...entity, kind: 'npc', icon: entity.role === 'enemy' ? 'sword' : entity.role === 'quest_giver' ? 'flag' : 'person'})),
    ...(room.items || []).map(entity => ({...entity, kind: 'item', icon: entity.obtainable ? 'bag' : 'landmark'})),
  ];
  const spots = [[325, 130], [575, 130], [325, 200], [575, 200], [325, 270], [575, 270]];
  return markers.slice(0, spots.length).map((marker, index) => ({...marker, x: spots[index][0], y: spots[index][1]}));
}

// Decorative palettes follow the server's prose; they do not add world paths.
export function getMapAppearance(room) {
  const prose = `${room.name || ''} ${room.description || ''}`;
  if (/sewer|cellar|cave|subterranean|underground/i.test(prose)) return 'underground';
  if (/ruin|bastion|fortification|vault/i.test(prose)) return 'ruins';
  if (/garden|willow|wild greenery/i.test(prose)) return 'garden';
  if (/tavern|hearth|innkeeper/i.test(prose)) return 'interior';
  if (/square|market|street|alley|village/i.test(prose)) return 'settlement';
  return 'wilds';
}
