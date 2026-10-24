# Catalog design

The Plugins and Skills modes extend the existing registry visual system: Inter
for interface copy, JetBrains Mono for commands, shared light/dark surface and
foreground tokens, and the existing cyan/purple accents. This change does not
introduce a separate brand or asset set.

Both modes use `CatalogSearchField` and `CatalogCard`. Skills installation uses
the existing `AppMultiSelect`, persisted installation preferences, and
`CommandSnippet`. Skills remain a separate data model from reviewed packages.
The mode is reflected in `?tab=skills`; search and sorting survive navigation.

The catalog heading combines a localized shared prefix with the Plugins/Skills
segmented switch as its final phrase. It is the directory's unique h1 and the
home catalog's h2. One introductory sentence follows; child catalogs do not
repeat a heading or intro. Collapsed Skill cards are compact; agent selection and commands appear after Install.
Agent choices come from the generated client capability manifest.

Catalog translations belong to the feature's compiled Vue I18n resources.
One local composer serves the catalog and its Skill cards; shared controls fall
back to global labels. Directory SEO remains global. Detail and download pages
do not serialize catalog-only copy into their HTML payloads.
Catalog-specific styles also load with the catalog component instead of the
global registry stylesheet, keeping unrelated pages within their HTML budget.

Desktop is the acceptance target. Review includes real skills.sh data, search,
installation choices, copy, navigation, pagination, and outage/empty states.
Cards link only to their validated GitHub repository roots. Provider links and
listing notices are absent. The collection count describes the current view,
and pagination remains below the cards. The CLI validates the repository and
exact declared Skill name.

Impeccable's final detector identified two inherited patterns in unrelated
registry styles (gradient text and a workflow accent stripe). They are outside
this catalog extension. The independent finish review requested removal of
duplicate introductory copy and excess collapsed-card spacing; both are fixed.
No raster assets were added; raster provenance is therefore not applicable.

The React CLI picker remains unchanged. A later migration can share these Vue
components through a small static entry point while retaining its local server
and installation contract.
