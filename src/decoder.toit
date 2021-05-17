// Copyright (C) 2021 Toitware ApS. All rights reserved.

import .message
import .reader

/**
Interface for all message decoders.
*/
interface Decoder:
  id -> int
  decode reader/Reader -> Message
