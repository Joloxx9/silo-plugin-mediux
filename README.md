# Mediux Artwork for Silo

Movie and series posters, backdrops, logos, season posters, and episode title cards from [Mediux](https://mediux.pro). Connects directly to Mediux; no separate container is needed.

## Install

1. Open Silo's administrator **Plugins** page, select **Catalog**, and click **Add repository**. Give it a name such as **Mediux** and use this URL:

   ```text
   https://raw.githubusercontent.com/Pukabyte/silo-plugin-mediux/main/index.json
   ```

2. In **Catalog**, find **Mediux Artwork** under **Other sources**, click **Install**, then enable it. If it does not appear immediately, reload the page and clear any catalog filters.

Silo downloads the correct binary and checks its checksum. Updates are available through the same repository. Existing ZIP installations are not automatically linked to a repository for updates.

Available for Linux amd64, Linux arm64, and macOS arm64. Linux amd64 is runtime-tested.

## Setup

1. Save your **Mediux API token** in the plugin's settings.
2. Add **Mediux Artwork** to the movie, series, season, and episode provider lists you use. Put it first when you want Mediux artwork. Keep your other metadata providers for title information and TMDB/TVDB IDs.
3. Open **Mediux artwork settings** from the plugin menu or administrator sidebar.
4. Choose your creators and artwork preferences, then save.
5. Refresh an item to apply your choices. Saving settings does not replace existing artwork.

Artwork preference changes save immediately without restarting Silo or entering your saved token again.

## Creator priority

Search for a Mediux username or add one manually. Use **Up/Down** to arrange creators; the top entry has highest priority.

- **Only use prioritized creators** restricts artwork to your list.
- **Excluded creators** are never selected.

Movie artwork is considered from both Mediux **movie sets** and **collection
sets**. A movie that a creator published only inside a collection set, such as
a franchise collection, competes on the same footing as one from a dedicated
movie set, so your creator order decides between them. Series have no
collections and are unaffected.

## Matching season posters and title cards

Season posters and title cards have separate consistency settings, so each can come from a different set.

| Setting | Selection |
| --- | --- |
| **Creator priority first; most artwork per creator** | Choose the highest-priority available creator, then their set with the most season posters or title cards. |
| **Complete set first; then creator priority** | Prefer a complete set, then creator order. If none is complete, use creator order, then coverage. This is the default. |
| **Match series poster set** | Use the same set as the automatically selected series poster. |
| **Choose each image separately** | Choose each image independently. |

Coverage compares distinct seasons or episodes in artwork available from Mediux. It does not inspect your local library. A manually chosen series poster cannot anchor later automatic set selection.

For missing artwork, choose whether to fall back to another Mediux set or leave it without a Mediux replacement. Other configured providers may still supply an image.

## Other artwork settings

- **Best image only** supplies automatic choices. On Silo versions supporting full Mediux galleries, the manual picker shows all matching images, while keeping creator exclusions and the **Only use prioritized creators** filter. Older Silo versions may still limit the picker to automatic choices.
- **All matching images** makes every eligible image available for hosts that do not yet distinguish manual picking from automatic refresh.
- **Optimized JPEG** uses smaller images. **Original** preserves source quality. Logos retain transparency.
- **Cache hours** controls how long Mediux lookup results are reused. Default: 24 hours. Set 0 to disable this cache.

Silo controls artwork storage and image caching. Your API token stays on the server and is never included in image URLs. Episode title cards are supplied during metadata refresh; manual image-picker availability depends on your Silo version.

## Support

[Report an issue](https://github.com/Pukabyte/silo-plugin-mediux/issues). For development, see [CONTRIBUTING.md](CONTRIBUTING.md).

[AGPL-3.0-or-later](LICENSE). Embedded Outfit font: [SIL OFL](OFL-Outfit.txt). Independently maintained; not affiliated with Mediux or Silo.
