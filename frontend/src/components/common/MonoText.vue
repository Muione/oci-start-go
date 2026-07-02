<template>
  <span class="mono-text">
    <slot />
  </span>
</template>

<style scoped>
.mono-text {
  font-family: var(--font-mono);
  font-size: 0.875em;
  letter-spacing: var(--tracking-mono);
}
</style>
