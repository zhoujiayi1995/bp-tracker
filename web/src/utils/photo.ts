const MAX_EDGE = 1600

// Re-encode through a canvas so phone HEIC becomes JPEG and EXIF orientation is applied.
export function preparePhoto(file: Blob): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const image = new Image()
    const fail = () => {
      URL.revokeObjectURL(url)
      reject(new Error('decode'))
    }
    image.onload = () => {
      URL.revokeObjectURL(url)
      const longest = Math.max(image.naturalWidth, image.naturalHeight)
      if (longest < 1) {
        reject(new Error('decode'))
        return
      }
      const scale = Math.min(1, MAX_EDGE / longest)
      const width = Math.max(1, Math.round(image.naturalWidth * scale))
      const height = Math.max(1, Math.round(image.naturalHeight * scale))
      const canvas = document.createElement('canvas')
      canvas.width = width
      canvas.height = height
      const ctx = canvas.getContext('2d')
      if (!ctx) {
        reject(new Error('decode'))
        return
      }
      ctx.drawImage(image, 0, 0, width, height)
      canvas.toBlob(
        (blob) => (blob ? resolve(blob) : reject(new Error('decode'))),
        'image/jpeg',
        0.85,
      )
    }
    image.onerror = fail
    image.src = url
  })
}
