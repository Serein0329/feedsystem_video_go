<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ text: string }>()

const parts = computed(() => props.text.split(/(#[\p{L}\p{N}_]+)/gu).filter((part) => part !== ''))

function isTag(part: string) {
  return part.startsWith('#')
}

function tagName(part: string) {
  return part.slice(1)
}
</script>

<template>
  <span>
    <template v-for="(part, index) in parts" :key="index">
      <RouterLink v-if="isTag(part)" class="tag-link" :to="`/tag/${encodeURIComponent(tagName(part))}`">{{ part }}</RouterLink>
      <span v-else>{{ part }}</span>
    </template>
  </span>
</template>

<style scoped>
.tag-link {
  color: rgba(43, 161, 255, 0.95);
  text-decoration: none;
}
.tag-link:hover {
  text-decoration: underline;
}
</style>
