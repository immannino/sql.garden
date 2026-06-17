import type { main } from '../../wailsjs/go/models'

// Desktop stub — never called when IS_DESKTOP=true.
export const WEB_DATASETS: main.SampleDataset[] = []
export const loadWebDataset = async (_id: string): Promise<main.CanvasAction[]> => []
