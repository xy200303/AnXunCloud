<template>
  <!-- 照片墙 + 查看器：缩略图（水印图），点击进入查看器（原图放大、左右切换、元数据） -->
  <div class="photo-wall">
    <div
      v-for="(p, i) in photos"
      :key="i"
      class="photo-thumb"
      @click="open(i)"
    >
      <el-image :src="p.watermarked_url || p.url" fit="cover" class="thumb-img" />
      <span class="thumb-item">{{ p.item || '照片' }}</span>
    </div>
    <span v-if="!photos.length" class="text-secondary">无照片</span>
  </div>

  <el-dialog v-model="viewerVisible" :title="current?.item || '照片'" width="720px" class="photo-viewer" append-to-body>
    <template v-if="current">
      <div class="viewer-body">
        <el-button class="viewer-nav" :icon="ArrowLeft" circle :disabled="index === 0" @click="index--" />
        <el-image
          :src="current.url"
          fit="contain"
          class="viewer-img"
          :preview-src-list="[]"
        />
        <el-button class="viewer-nav" :icon="ArrowRight" circle :disabled="index === photos.length - 1" @click="index++" />
      </div>
      <div class="viewer-meta">
        <span>{{ index + 1 }} / {{ photos.length }}</span>
        <span v-if="meta?.time">打卡时间 {{ meta.time }}</span>
        <span v-if="meta?.distance != null">距点位 {{ meta.distance }}m</span>
        <span v-if="meta?.coords">坐标 {{ meta.coords }}</span>
        <!-- EXIF 校验结论 -->
        <template v-if="current.exif_check">
          <el-tag v-if="current.exif_check.passed === true" type="success" size="small">
            EXIF 校验通过（偏差 {{ current.exif_check.deviation_seconds }}s）
          </el-tag>
          <el-tag v-else-if="current.exif_check.passed === false" type="danger" size="small">
            EXIF 校验未通过（偏差 {{ current.exif_check.deviation_seconds }}s）
          </el-tag>
          <el-tag v-else type="info" size="small">无 EXIF 信息</el-tag>
        </template>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { ArrowLeft, ArrowRight } from '@element-plus/icons-vue'

// 照片项：打卡照片有 item/exif_check；工单照片仅 url/watermarked_url
export interface PhotoItem {
  item?: string
  url: string
  watermarked_url?: string | null
  exif_check?: { shot_at: string | null; deviation_seconds: number | null; passed: boolean | null } | null
}

const props = defineProps<{
  photos: PhotoItem[]
  // 打卡元数据（时间/距离/坐标），查看器底部展示
  meta?: { time?: string; distance?: number; coords?: string }
}>()

const viewerVisible = ref(false)
const index = ref(0)

const current = computed(() => props.photos[index.value])

function open(i: number) {
  index.value = i
  viewerVisible.value = true
}
</script>

<style scoped lang="scss">
.photo-wall {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-sm;
}

.photo-thumb {
  width: 88px;
  cursor: pointer;
  border-radius: $radius-small;
  overflow: hidden;
  border: 1px solid $color-border;

  .thumb-img {
    width: 88px;
    height: 88px;
    display: block;
  }

  .thumb-item {
    display: block;
    font-size: $font-size-aux;
    color: $color-text-secondary;
    text-align: center;
    padding: 2px 4px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}

.viewer-body {
  display: flex;
  align-items: center;
  gap: $spacing-md;

  .viewer-img {
    flex: 1;
    height: 420px;
    background: $color-bg-page;
    border-radius: $radius-small;
  }
}

.viewer-meta {
  display: flex;
  align-items: center;
  gap: $spacing-lg;
  margin-top: $spacing-md;
  font-size: $font-size-aux;
  color: $color-text-secondary;
  flex-wrap: wrap;
}
</style>
