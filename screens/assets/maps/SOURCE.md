# AIS base map

`ais-world.png` was rendered from Natural Earth `ne_50m_admin_0_countries.geojson`.
Natural Earth data is public domain: https://www.naturalearthdata.com/about/terms-of-use/
Source: https://github.com/nvkelso/natural-earth-vector

The source GeoJSON and deterministic generator are kept in the repository.

The ADS-B map also loads Web Mercator tiles when available. The primary source
is OpenStreetMap's German community tile server
(`https://tile.openstreetmap.de/{z}/{x}/{y}.png`), with Esri World Street Map
as a fallback (`https://server.arcgisonline.com/ArcGIS/rest/services/World_Street_Map/MapServer/tile/{z}/{y}/{x}`).
Tiles are cached under `DATA/cache/map-tiles-v2/`. The bundled Natural Earth
image above remains visible when tiles are unavailable. Attribution is shown
on the ADS-B map.
