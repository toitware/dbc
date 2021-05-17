import .decoder
import .message
import .reader

export *

/**
Dispatcher for DBC-encoded messages, to the individual callbacks.
*/
class Dispatcher:
  callbacks_ ::= {:}

  /**
  Registers a decoder, with the associated callback.

  The callback is called with the decoded message.
  */
  register decoder/Decoder callback/Lambda:
    callbacks_[decoder.id] = CallbackState_ decoder callback

  /**
  Dispatches the decoded message to the right callback.

  Returns `true` if a callback was registered for this message
  */
  dispatch id/int data/ByteArray -> bool:
    callbacks_.get id --if_present=: | state/CallbackState_ |
      reader := Reader data
      msg := state.decoder.decode reader
      state.callback.call msg
      return true
    return false

class CallbackState_:
  decoder/Decoder
  callback/Lambda
  constructor .decoder .callback:
