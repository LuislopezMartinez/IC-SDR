# Maps

Live ADS-B and AIS maps use Web Mercator raster tiles from CARTO Voyager
(`https://basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}.png`).
That style keeps country, state/region and city labels at street-level zoom
instead of stretching a single world bitmap.

Tiles are cached under `DATA/cache/map-tiles/{z}/{x}/{y}.png`. Requests send
`User-Agent: IC-SDR/0.5 (+https://github.com/blkph0x/IC-SDR)`.

Attribution shown on the maps: © OpenStreetMap · © CARTO.

`ais-world.png` remains as an offline fallback at wide zoom. It was rendered
from Natural Earth `ne_50m_admin_0_countries.geojson` (public domain):
https://www.naturalearthdata.com/about/terms-of-use/
Source: https://github.com/nvkelso/natural-earth-vector
