import JSEncrypt from 'jsencrypt'

// encryptPassword RSA-PKCS1v15-encrypts the plaintext with the server's public
// key (base64 SPKI from /api/login/init) and returns base64 ciphertext. The
// Go server decrypts with crypto/rsa DecryptPKCS1v15 — padding matches.
export function encryptPassword(plain: string, pubKeyB64: string): string {
  const enc = new JSEncrypt()
  // jsencrypt expects PEM; wrap the base64 SPKI.
  const chunks = (pubKeyB64.match(/.{1,64}/g) || [pubKeyB64]).join('\n')
  const pem = `-----BEGIN PUBLIC KEY-----\n${chunks}\n-----END PUBLIC KEY-----`
  enc.setPublicKey(pem)
  const out = enc.encrypt(plain)
  if (out === false || out == null) {
    throw new Error('RSA 加密失败')
  }
  return out
}
