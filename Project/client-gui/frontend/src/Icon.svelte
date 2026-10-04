<script>
  export let name = 'map';
  export let size = 20;
  // Solid 16×16 icons: no curves, thin strokes or icon dependency.
  const paths = {
    map: 'M1 2h4v2H3v9h2v1H1zM6 4h4v9H6zM11 2h4v12h-4v-2h2V4h-2z',
    eye: 'M5 3h6v2H5zM3 5h2v2H3zM1 7h2v2H1zM3 9h2v2H3zM5 11h6v2H5zM11 5h2v2h-2zM13 7h2v2h-2zM11 9h2v2h-2zM6 6h4v4H6z',
    heart: 'M2 3h4v1h4V3h4v2h1v5h-2v2h-2v2H5v-2H3v-2H1V5h1z',
    book: 'M2 2h5v1h2V2h5v12H9v1H7v-1H2zM7 4v8h2V4z',
    users: 'M2 2h4v4H2zM10 2h4v4h-4zM1 8h6v6H1zM9 8h6v6H9z',
    flag: 'M2 1h2v14H2zM5 2h9v7H5z',
    bag: 'M5 1h6v2H5zM3 3h2v2H3zM11 3h2v2h-2zM2 5h12v9H2z',
    person: 'M5 1h6v6H5zM3 9h10v6H3z',
    sword: 'M11 1h4v4h-2v2h-2v2H9v2H7v2H5v2H1v-4h2V9h2V7h2V5h2V3h2zM2 6h2v2H2zM8 12h2v2H8z',
    shield: 'M2 2h12v8h-2v2h-2v2H6v-2H4v-2H2z',
    pin: 'M4 1h8v2h2v7h-2v2h-2v2H6v-2H4v-2H2V3h2zM6 4v4h4V4z',
    expand: 'M1 1h5v2H3v3H1zM10 1h5v5h-2V3h-3zM1 10h2v3h3v2H1zM13 10h2v5h-5v-2h3z',
    close: 'M2 2h3v3H2zM5 5h2v2H5zM7 7h2v2H7zM9 9h2v2H9zM11 11h3v3h-3zM11 2h3v3h-3zM9 5h2v2H9zM5 9h2v2H5zM2 11h3v3H2z',
    arrow: 'M7 2h3v2h2v2h2v4h-2v2h-2v2H7v-4H1V6h6z',
    refresh: 'M4 2h8v2H4zM2 4h2v6H2zM4 10h8v2H4zM12 4h2v4h-5V6h3zM12 12h2v2h-2z',
    landmark: 'M6 1h4v2h2v2h2v2H2V5h2V3h2zM3 8h2v4H3zM7 8h2v4H7zM11 8h2v4h-2zM2 13h12v2H2z',
  };
</script>

<svg width={size} height={size} viewBox="0 0 16 16" fill="currentColor" fill-rule="evenodd" shape-rendering="crispEdges" aria-hidden="true"><path d={paths[name] || paths.map} /></svg>
