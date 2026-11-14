<script setup lang="ts">
const { content } = useLandingContent()
const { t } = useI18n()

type ComparisonKey = "pluginKitAi" | "manual" | "duplicated" | "scripts"

const columns: Array<{ key: ComparisonKey; labelKey: string; highlight?: boolean }> = [
  { key: "pluginKitAi", labelKey: "comparison.columns.pluginKitAi", highlight: true },
  { key: "manual", labelKey: "comparison.columns.manual" },
  { key: "duplicated", labelKey: "comparison.columns.duplicated" },
  { key: "scripts", labelKey: "comparison.columns.scripts" }
]

function getStatusIcon(status: "yes" | "partial" | "no"): string {
  switch (status) {
    case "yes":
      return "✓"
    case "partial":
      return "◐"
    default:
      return "✕"
  }
}

function getCellClass(status: "yes" | "partial" | "no"): string {
  switch (status) {
    case "yes":
      return "comparison-row__cell--yes"
    case "partial":
      return "comparison-row__cell--partial"
    default:
      return "comparison-row__cell--no"
  }
}

function getCell(
  row: (typeof content.value.comparisonRows)[number],
  key: ComparisonKey
) {
  return row[key]
}
</script>

<template>
  <section id="comparison" class="comparison-section section anchor-offset">
    <v-container>
      <div class="comparison-section__header">
        <h2 class="comparison-section__title">
          {{ t("comparison.sectionTitle") }}
        </h2>
        <p class="comparison-section__subtitle">
          {{ t("comparison.sectionSubtitle") }}
        </p>
      </div>

      <div class="comparison-grid">
        <div class="comparison-grid__header">
          <div class="comparison-grid__feature-head">
            {{ t("comparison.feature") }}
          </div>
          <div
            v-for="column in columns"
            :key="column.key"
            class="comparison-grid__column-head"
            :class="{ 'comparison-grid__column-head--highlight': column.highlight }"
          >
            {{ t(column.labelKey) }}
          </div>
        </div>

        <article
          v-for="row in content.comparisonRows"
          :key="row.id"
          class="comparison-row"
        >
          <div class="comparison-row__feature">
            {{ row.feature }}
          </div>

          <div class="comparison-row__cells">
            <div
              v-for="column in columns"
              :key="column.key"
              class="comparison-row__cell"
              :class="[
                getCellClass(getCell(row, column.key).status),
                { 'comparison-row__cell--highlight': column.highlight }
              ]"
            >
              <span class="comparison-row__icon">
                {{ getStatusIcon(getCell(row, column.key).status) }}
              </span>
              <span class="comparison-row__note">
                {{ getCell(row, column.key).note }}
              </span>
            </div>
          </div>
        </article>
      </div>
    </v-container>
  </section>
</template>

<style scoped>
.comparison-section {
  position: relative;
}

.comparison-section__header {
  text-align: center;
  max-width: 720px;
  margin: 0 auto 56px;
  position: relative;
  z-index: 1;
}

.comparison-section__title {
  font-size: 2.4rem;
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 1.15;
  margin-bottom: 16px;
  background: linear-gradient(135deg, #e0e6ff 0%, #39ff14 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.comparison-section__subtitle {
  font-size: 1.08rem;
  color: #8892b0;
  line-height: 1.65;
  margin: 0;
}

.comparison-grid {
  border-radius: 24px;
  border: 1px solid rgba(0, 240, 255, 0.12);
  background:
    linear-gradient(180deg, rgba(11, 15, 25, 0.92), rgba(8, 11, 18, 0.84));
  box-shadow:
    0 18px 60px rgba(0, 0, 0, 0.22),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
  backdrop-filter: blur(16px);
  overflow: hidden;
}

.comparison-grid__header {
  display: grid;
  grid-template-columns: minmax(220px, 1.15fr) repeat(4, minmax(0, 1fr));
  gap: 0;
}

.comparison-grid__feature-head,
.comparison-grid__column-head {
  padding: 18px 20px;
  font-size: 0.78rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: #9aa7c7;
  font-family: "JetBrains Mono", monospace;
  background: rgba(255, 255, 255, 0.02);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.comparison-grid__column-head--highlight {
  background: rgba(0, 240, 255, 0.04);
}

.comparison-row {
  display: grid;
  grid-template-columns: minmax(220px, 1.15fr) minmax(0, 4fr);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.comparison-row:last-child {
  border-bottom: none;
}

.comparison-row__feature {
  padding: 20px;
  color: #e2e8f0;
  font-weight: 600;
  line-height: 1.5;
}

.comparison-row__cells {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.comparison-row__cell {
  display: grid;
  grid-template-columns: 20px minmax(0, 1fr);
  gap: 10px;
  align-items: start;
  padding: 20px 18px;
}

.comparison-row__cell--highlight {
  background: rgba(0, 240, 255, 0.04);
}

.comparison-row__icon {
  font-weight: 800;
  line-height: 1.1;
}

.comparison-row__note {
  color: #b9c4e3;
  line-height: 1.55;
  font-size: 0.92rem;
}

.comparison-row__cell--yes .comparison-row__icon {
  color: #39ff14;
}

.comparison-row__cell--partial .comparison-row__icon {
  color: #ffd166;
}

.comparison-row__cell--no .comparison-row__icon {
  color: #ff6b6b;
}

.v-theme--light .comparison-section__title {
  background: linear-gradient(135deg, #0f172a 0%, #15803d 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.v-theme--light .comparison-section__subtitle {
  color: #475569;
}

.v-theme--light .comparison-grid {
  border-color: rgba(15, 23, 42, 0.08);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.95), rgba(248, 250, 252, 0.94));
  box-shadow:
    0 18px 50px rgba(15, 23, 42, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.9);
}

.v-theme--light .comparison-grid__feature-head,
.v-theme--light .comparison-grid__column-head {
  color: #64748b;
  background: rgba(148, 163, 184, 0.06);
  border-bottom-color: rgba(15, 23, 42, 0.06);
}

.v-theme--light .comparison-grid__column-head--highlight,
.v-theme--light .comparison-row__cell--highlight {
  background: rgba(14, 165, 233, 0.05);
}

.v-theme--light .comparison-row {
  border-bottom-color: rgba(15, 23, 42, 0.06);
}

.v-theme--light .comparison-row__feature {
  color: #0f172a;
}

.v-theme--light .comparison-row__note {
  color: #475569;
}

@media (max-width: 960px) {
  .comparison-section__title {
    font-size: 1.85rem;
  }

  .comparison-section__header {
    margin-bottom: 40px;
  }

  .comparison-grid {
    overflow-x: auto;
  }

  .comparison-grid__header,
  .comparison-row {
    min-width: 920px;
  }
}

@media (max-width: 600px) {
  .comparison-section__title {
    font-size: 1.6rem;
  }
}
</style>
