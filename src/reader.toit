// Copyright (C) 2021 Toitware ApS. All rights reserved.

/**
Bit-based Reader for reading integer values at arbritary offsets and widths, within
  a byte array.
*/
class Reader:
  data_/ByteArray

  constructor .data_:

  read bit_offset/int bit_size/int --signed=false:
    bits := read_bits_ bit_offset bit_size
    if signed: bits = (bits << (64 - bit_size)) >> (64 - bit_size)
    return bits

  read_bits_ bit_offset/int bit_size/int:
    offset_in_byte := bit_offset & 0b111
    result := 0
    result_width := 0
    while bit_size > 0:
      byte := data_[bit_offset >> 3]
      byte_width := 8 - offset_in_byte
      value := ?
      value_width := ?
      if (offset_in_byte + bit_size) >= 8:
        // Read rest of byte.
        value = byte >> offset_in_byte
        value_width = byte_width
      else:
        // Read part of byte.
        byte_shifted := byte >> offset_in_byte
        value = byte_shifted & ~(0xff << bit_size)
        value_width = bit_size

      result |= value << result_width
      result_width += value_width

      bit_size -= byte_width
      bit_offset += byte_width
      offset_in_byte = 0

    return result

/**
Converts the raw value to the physical values, as describes by the parameters.
*/
to_physical raw/int factor/num offset/num min_value/num max_value/num -> num:
  value := raw * factor + offset
  if min_value == max_value: return value
  return min max_value (max min_value value)
