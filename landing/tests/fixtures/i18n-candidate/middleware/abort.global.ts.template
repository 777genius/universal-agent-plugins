export default defineNuxtRouteMiddleware(() => {
  const abortNext = useState('fixture:abort', () => false);
  if (abortNext.value) {
    abortNext.value = false;
    return abortNavigation();
  }
});
