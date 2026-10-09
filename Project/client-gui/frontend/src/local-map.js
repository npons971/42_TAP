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
  const spots = [[355, 193], [545, 193], [360, 244], [540, 244], [392, 147], [508, 147]];
  return markers.slice(0, spots.length).map((marker, index) => ({...marker, x: spots[index][0], y: spots[index][1]}));
}

// Decorative palettes follow the server's prose; they do not add world paths.
export function getMapAppearance(room) {
  const prose = `${room.name || ''} ${room.description || ''}`;
  if (/sewer|cellar|cave|subterranean|underground/i.test(prose)) return {kind: 'underground', ground: '#303e45', floor: '#59666a'};
  if (/ruin|bastion|fortification|vault/i.test(prose)) return {kind: 'ruins', ground: '#45483b', floor: '#7c7765'};
  if (/garden|willow|wild greenery/i.test(prose)) return {kind: 'garden', ground: '#3e6045', floor: '#698258'};
  if (/tavern|hearth|innkeeper/i.test(prose)) return {kind: 'interior', ground: '#4c4236', floor: '#837355'};
  if (/square|market|street|alley|village/i.test(prose)) return {kind: 'settlement', ground: '#344e41', floor: '#697263'};
  return {kind: 'wilds', ground: '#344e41', floor: '#697263'};
}
