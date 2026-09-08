import { slavicPluralRule } from './utils/slavicPluralRule';
export default defineI18nConfig(() => ({
  pluralRules: { ru: slavicPluralRule, uk: slavicPluralRule },
}));
