<script setup lang="ts">
// Vue Internals proving fixture (issue #2876).
// Exercises: composition_api (ref/computed/watch), props_emits_macros
// (defineProps/defineEmits/defineExpose), provide_inject (provide/inject),
// pinia_store (defineStore import + useXxxStore), sfc_block_extraction
// (this <script setup> + <template> + <style scoped> split).
import { ref, computed, watch, provide, inject } from 'vue'
import { useCounterStore } from '@/stores/counter'

const props = defineProps<{ label: string; max?: number }>()
const emit = defineEmits<{ (e: 'change', value: number): void }>()

const counterStore = useCounterStore()
const count = ref(0)
const doubled = computed(() => count.value * 2)
watch(count, (n) => emit('change', n))

provide('theme', 'dark')
const injectedUser = inject('currentUser')

defineExpose({ count })
</script>

<template>
  <section>
    <!-- directive_recognition: v-for, v-model, v-bind shorthand, v-on shorthand -->
    <input v-model="count" :max="props.max" @input="emit('change', count)" />
    <li v-for="item in items" :key="item.id">{{ item.name }}</li>
    <ChildPanel v-if="doubled > 0">
      <!-- slot_extraction: content provided to a child's named slot -->
      <template #header>{{ props.label }}</template>
      <template v-slot:footer>{{ injectedUser }}</template>
    </ChildPanel>
    <!-- slot_extraction: this component's own slot outlets -->
    <slot name="title" />
    <slot />
  </section>
</template>

<style scoped>
/* scoped_style_extraction is N/A — CSS is ignored by the extractor by design. */
.section { color: red; }
</style>
