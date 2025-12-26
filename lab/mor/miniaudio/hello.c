#include <string.h>
#include "miniaudio.h"

void data_callback(ma_device* pDevice, void* pOutput, const void* pInput, ma_uint32 frameCount)
{
    if (pDevice->capture.format != pDevice->playback.format || pDevice->capture.channels != pDevice->playback.channels) {
        return;
    }

    memcpy(pOutput, pInput, frameCount * ma_get_bytes_per_frame(pDevice->capture.format, pDevice->capture.channels));
}
