<script setup lang="ts">
import { ref } from 'vue'
import { Check, Copy } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { copyText } from '@/composables/clipboard'
import { cn } from '@/lib/utils'

const props = defineProps<{ value: string; class?: string }>()
const copied = ref(false)
let resetTimer: ReturnType<typeof setTimeout> | undefined

async function copy(): Promise<void> {
  await copyText(props.value)
  copied.value = true
  clearTimeout(resetTimer)
  resetTimer = setTimeout(() => (copied.value = false), 1500)
}
</script>

<template>
  <Tooltip>
    <TooltipTrigger as-child>
      <Button
        variant="ghost"
        size="icon-sm"
        :class="cn('text-muted-foreground hover:text-foreground size-7 shrink-0', props.class)"
        :aria-label="$t('copy')"
        @click="copy"
      >
        <Check v-if="copied" class="text-success" />
        <Copy v-else />
      </Button>
    </TooltipTrigger>
    <TooltipContent>{{ copied ? $t('copied') : $t('click_to_copy') }}</TooltipContent>
  </Tooltip>
</template>
