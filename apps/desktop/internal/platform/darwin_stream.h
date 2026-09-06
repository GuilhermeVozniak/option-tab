#include <stdint.h>
void ot_stream_start(uint64_t token, uint32_t window, int maxpx, int pid, uint64_t startSec, uint64_t startUsec);
void ot_stream_stop(uint64_t token);
