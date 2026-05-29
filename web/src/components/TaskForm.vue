<template>
  <el-form :model="form" label-width="120px" label-position="top">
    <el-form-item label="邮箱模式">
      <el-radio-group v-model="form.emailMode">
        <el-radio value="moemail">MoeMail</el-radio>
        <el-radio value="cloudflare">Cloudflare Temp Email</el-radio>
        <el-radio value="outlook">Outlook</el-radio>
      </el-radio-group>
    </el-form-item>

    <el-form-item label="注册数量">
      <el-input-number v-model="form.count" :min="1" :max="1000" />
    </el-form-item>

    <el-form-item label="并发数">
      <el-input-number v-model="form.concurrency" :min="1" :max="10" />
    </el-form-item>

    <el-form-item label="延迟 (秒)">
      <el-input-number v-model="form.delay" :min="0" :max="60" />
    </el-form-item>

    <el-form-item label="代理地址">
      <el-input v-model="form.proxy" placeholder="留空使用全局配置" />
    </el-form-item>

    <el-form-item label="二级代理 (住宅IP)">
      <el-input v-model="form.upstreamProxy" placeholder="留空使用全局配置" />
    </el-form-item>

    <template v-if="form.emailMode === 'moemail'">
      <el-form-item label="MoeMail URL">
        <el-input v-model="form.moEmailUrl" placeholder="留空使用全局配置" />
      </el-form-item>
      <el-form-item label="MoeMail API Key">
        <el-input v-model="form.moEmailKey" placeholder="留空使用全局配置" />
      </el-form-item>
    </template>

    <template v-if="form.emailMode === 'cloudflare'">
      <el-form-item label="CF Email URL">
        <el-input v-model="form.cfEmailUrl" placeholder="留空使用全局配置" />
      </el-form-item>
      <el-form-item label="CF Admin Auth">
        <el-input v-model="form.cfEmailAuth" placeholder="留空使用全局配置" show-password />
      </el-form-item>
    </template>

    <el-form-item>
      <el-button type="primary" @click="handleSubmit">创建任务</el-button>
      <el-button @click="emit('cancel')">取消</el-button>
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { reactive } from 'vue'
import type { TaskForm } from '../api'

const emit = defineEmits<{
  submit: [form: TaskForm]
  cancel: []
}>()

const form = reactive<TaskForm>({
  count: 1,
  concurrency: 1,
  delay: 0,
  proxy: '',
  upstreamProxy: '',
  emailMode: 'moemail',
  outlookCsv: '',
  moEmailUrl: '',
  moEmailKey: '',
  cfEmailUrl: '',
  cfEmailAuth: '',
})

function handleSubmit() {
  emit('submit', { ...form })
}
</script>
