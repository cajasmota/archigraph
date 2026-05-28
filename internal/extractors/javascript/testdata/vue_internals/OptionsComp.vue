<script lang="ts">
// Vue Internals proving fixture (issue #2876) — Options API variant.
// Exercises: options_api (export default options object with a method block),
// provide_inject (provide/inject options), props (Options-API props).
import { defineComponent } from 'vue'

export default defineComponent({
  name: 'OptionsComp',
  props: {
    title: String,
    count: Number,
  },
  provide() {
    return { theme: 'light' }
  },
  inject: ['currentUser'],
  methods: {
    increment() {
      this.count += 1
    },
    reset() {
      this.count = 0
    },
  },
})
</script>

<template>
  <div>{{ title }}</div>
</template>
