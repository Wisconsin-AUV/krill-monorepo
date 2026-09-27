import { createContext, useContext } from 'react'

export const FieldInvalid = createContext(false)

export function useFieldInvalid() {
  return useContext(FieldInvalid)
}
