// 腾讯地图 GL（全局 script 注入，无官方类型包）最小类型声明：
// 仅覆盖 MapPickerDialog 实际用到的 API 面，未用到的能力不声明

interface TMapLatLng {
  getLat(): number
  getLng(): number
}

interface TMapMapEvent {
  latLng?: TMapLatLng
}

interface TMapMap {
  setCenter(latLng: TMapLatLng): void
  setZoom(zoom: number): void
  setDraggable(draggable: boolean): void
  removeControl(control: unknown): void
  on(event: string, handler: (evt: TMapMapEvent) => void): void
  destroy(): void
}

interface TMapMultiMarker {
  setGeometries(geometries: { id?: string; position?: TMapLatLng }[]): void
  setStopPropagation(stop: boolean): void
  on(event: string, handler: () => void): void
  setMap(map: TMapMap | null): void
}

interface TMapNamespace {
  LatLng: new (lat: number, lng: number) => TMapLatLng
  Map: new (el: HTMLElement, opts: { center?: TMapLatLng; zoom?: number; viewMode?: string }) => TMapMap
  MultiMarker: new (opts: { id?: string; map?: TMapMap; geometries?: { id?: string; position?: TMapLatLng }[] }) => TMapMultiMarker
  constants: { DEFAULT_CONTROL_ID: Record<string, unknown> }
}

interface Window {
  TMap?: TMapNamespace
}
