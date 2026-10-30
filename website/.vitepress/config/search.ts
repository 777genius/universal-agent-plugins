// The local index is generated separately for each published locale by VitePress.
const copy: Record<string, string[]> = {
  en: ["Search", "Search documentation", "Display detailed list", "Reset search", "Close search", "No results for", "select", "Enter", "navigate", "Up arrow", "Down arrow", "close", "Escape"],
  ru: ["Поиск", "Поиск по документации", "Показать подробный список", "Очистить поиск", "Закрыть поиск", "Ничего не найдено по запросу", "выбрать", "Enter", "перейти", "Стрелка вверх", "Стрелка вниз", "закрыть", "Escape"],
  uk: ["Пошук", "Пошук у документації", "Показати докладний список", "Очистити пошук", "Закрити пошук", "Нічого не знайдено за запитом", "вибрати", "Enter", "перейти", "Стрілка вгору", "Стрілка вниз", "закрити", "Escape"],
  zh: ["搜索", "搜索文档", "显示详细列表", "清除搜索", "关闭搜索", "未找到结果", "选择", "回车键", "切换", "向上键", "向下键", "关闭", "Esc 键"],
  es: ["Buscar", "Buscar en la documentación", "Mostrar lista detallada", "Borrar búsqueda", "Cerrar búsqueda", "Sin resultados para", "seleccionar", "Intro", "navegar", "Flecha arriba", "Flecha abajo", "cerrar", "Escape"],
  hi: ["खोजें", "दस्तावेज़ों में खोजें", "विस्तृत सूची दिखाएँ", "खोज साफ़ करें", "खोज बंद करें", "कोई परिणाम नहीं मिला", "चुनें", "Enter", "आगे बढ़ें", "ऊपर तीर", "नीचे तीर", "बंद करें", "Escape"],
  ar: ["بحث", "البحث في الوثائق", "عرض القائمة المفصلة", "مسح البحث", "إغلاق البحث", "لا توجد نتائج لـ", "اختيار", "Enter", "تنقّل", "السهم لأعلى", "السهم لأسفل", "إغلاق", "Escape"],
  pt: ["Buscar", "Buscar na documentação", "Mostrar lista detalhada", "Limpar busca", "Fechar busca", "Nenhum resultado para", "selecionar", "Enter", "navegar", "Seta para cima", "Seta para baixo", "fechar", "Escape"],
  fr: ["Rechercher", "Rechercher dans la documentation", "Afficher la liste détaillée", "Effacer la recherche", "Fermer la recherche", "Aucun résultat pour", "sélectionner", "Entrée", "naviguer", "Flèche vers le haut", "Flèche vers le bas", "fermer", "Échap"]
};
export const searchLocales = Object.fromEntries(Object.entries(copy).map(([code, text]) => [code, {
  translations: {
    button: { buttonText: text[0], buttonAriaLabel: text[1] },
    modal: {
      displayDetails: text[2], resetButtonTitle: text[3], backButtonTitle: text[4], noResultsText: text[5],
      footer: { selectText: text[6], selectKeyAriaLabel: text[7], navigateText: text[8], navigateUpKeyAriaLabel: text[9], navigateDownKeyAriaLabel: text[10], closeText: text[11], closeKeyAriaLabel: text[12] }
    }
  }
}]));
