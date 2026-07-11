<template>
  <label class="oneui-switch" :class="{ 'is-disabled': disabled }">
    <input type="checkbox" class="oneui-switch-input" :checked="modelValue" :disabled="disabled" @change="onChange" />
    <span class="oneui-switch-track" aria-hidden="true">
      <span class="oneui-switch-thumb"></span>
    </span>
  </label>
</template>

<script setup>
const props = defineProps({
  modelValue: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
});
const emit = defineEmits(['update:modelValue', 'change']);
function onChange(event) {
  const next = event.target.checked;
  // Stay strictly controlled: revert the native toggle immediately so the switch only moves
  // when the parent actually updates modelValue. This keeps a guarded toggle in place when
  // its confirmation is cancelled (the parent never changes modelValue).
  event.target.checked = props.modelValue;
  emit('update:modelValue', next);
  emit('change', next);
}
</script>
