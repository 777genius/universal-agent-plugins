# Catalog design

The Plugins and Skills modes extend the existing registry visual system: Inter
for interface copy, JetBrains Mono for commands, shared light/dark surface and
foreground tokens, and the existing cyan/purple accents. This change does not
introduce a separate brand or asset set.

Both modes use `CatalogSearchField` and `CatalogCard`. Skills installation uses
the existing `AppMultiSelect`, persisted installation preferences, and
`CommandSnippet`. Skills remain a separate data model from reviewed packages.
The mode is reflected in `?tab=skills`; search and sorting survive navigation.

The directory's page heading already introduces Skills, so its catalog heading
is available to assistive technology without repeating visible introductory
copy. On the home page the catalog keeps its own visible heading. Collapsed
Skill cards are compact; agent selection and commands appear after Install.
Agent choices come from the generated client capability manifest.

Catalog translations belong to the feature's compiled Vue I18n resources.
One local composer serves the catalog and its Skill cards; shared controls fall
back to global labels. Directory SEO remains global. Detail and download pages
do not serialize catalog-only copy into their HTML payloads.
Catalog-specific styles also load with the catalog component instead of the
global registry stylesheet, keeping unrelated pages within their HTML budget.

Desktop is the acceptance target. Review includes real skills.sh data, search,
installation choices, copy, navigation, pagination, and outage/empty states.
The source and provider links identify provenance without claiming compatibility
or review. The CLI validates the repository and exact declared Skill name.

Impeccable's final detector identified two inherited patterns in unrelated
registry styles (gradient text and a workflow accent stripe). They are outside
this catalog extension. The independent finish review requested removal of
duplicate introductory copy and excess collapsed-card spacing; both are fixed.
No raster assets were added; raster provenance is therefore not applicable.

The React CLI picker remains unchanged. A later migration can share these Vue
components through a small static entry point while retaining its local server
and installation contract.
