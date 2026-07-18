import * as AppleAuthentication from 'expo-apple-authentication';
import { StatusBar } from 'expo-status-bar';
import React, { useEffect, useState } from 'react';
import {
  ActivityIndicator,
  Image,
  KeyboardAvoidingView,
  Linking,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { DEFAULT_BASE_URL } from '@/api/client';
import { useAuth } from '@/auth/AuthContext';
import { APPLE_AUTH_ENABLED } from '@/features';

const LOGO = require('../assets/logo-light.png');

function cleanUrl(value: string) {
  return value.trim().replace(/\/+$/, '');
}

function errorMessage(error: unknown) {
  if (error instanceof Error && error.message) return error.message;
  return '登录失败，请检查服务地址和账号信息后重试';
}

export default function LoginScreen() {
  const insets = useSafeAreaInsets();
  const {
    login,
    loginWithApple,
    savedEmail,
    savedPassword,
    baseUrl,
    basicAuth,
    updateBaseUrl,
    updateBasicAuth,
  } = useAuth();
  const [email, setEmail] = useState(savedEmail);
  const [password, setPassword] = useState(savedPassword);
  const [serverUrl, setServerUrl] = useState(baseUrl || DEFAULT_BASE_URL);
  const [basicAuthInput, setBasicAuthInput] = useState(basicAuth);
  const [showPassword, setShowPassword] = useState(false);
  const [showServer, setShowServer] = useState(false);
  const [agreed, setAgreed] = useState(Boolean(savedPassword));
  const [appleAvailable, setAppleAvailable] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!APPLE_AUTH_ENABLED || Platform.OS !== 'ios') return;
    AppleAuthentication.isAvailableAsync().then(setAppleAvailable).catch(() => setAppleAvailable(false));
  }, []);

  const applyServerSettings = async () => {
    const nextUrl = cleanUrl(serverUrl || DEFAULT_BASE_URL);
    if (!/^https?:\/\//i.test(nextUrl)) throw new Error('服务地址必须以 http:// 或 https:// 开头');
    await updateBaseUrl(nextUrl);
    await updateBasicAuth(basicAuthInput);
    return nextUrl;
  };

  const onLogin = async () => {
    if (!agreed) {
      setError('请先阅读并同意用户协议和隐私政策');
      return;
    }
    if (!email.trim() || !password) {
      setError('请输入邮箱和密码');
      return;
    }
    setBusy(true);
    setError('');
    try {
      const targetUrl = await applyServerSettings();
      await login(email, password, targetUrl);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const onAppleLogin = async () => {
    if (!agreed) {
      setError('请先阅读并同意用户协议和隐私政策');
      return;
    }
    setBusy(true);
    setError('');
    try {
      await applyServerSettings();
      const credential = await AppleAuthentication.signInAsync({
        requestedScopes: [
          AppleAuthentication.AppleAuthenticationScope.FULL_NAME,
          AppleAuthentication.AppleAuthenticationScope.EMAIL,
        ],
      });
      if (!credential.identityToken) throw new Error('Apple 登录未返回身份令牌');
      const fullName = [credential.fullName?.givenName, credential.fullName?.familyName].filter(Boolean).join(' ');
      await loginWithApple({
        identity_token: credential.identityToken,
        authorization_code: credential.authorizationCode || undefined,
        full_name: fullName || undefined,
      });
    } catch (e) {
      if ((e as { code?: string })?.code !== 'ERR_REQUEST_CANCELED') setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const openPolicy = (path: string) => Linking.openURL(`${cleanUrl(serverUrl || DEFAULT_BASE_URL)}${path}`).catch(() => undefined);

  return (
    <KeyboardAvoidingView style={styles.root} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <StatusBar style="light" />
      <ScrollView contentContainerStyle={[styles.content, { paddingTop: insets.top + 40, paddingBottom: insets.bottom + 32 }]} keyboardShouldPersistTaps="handled">
        <View style={styles.brandRow}>
          <Image source={LOGO} style={styles.logo} resizeMode="contain" />
          <View>
            <Text style={styles.brand}>DevLoom</Text>
            <Text style={styles.subtitle}>智能开发平台</Text>
          </View>
        </View>

        <Text style={styles.title}>欢迎回来</Text>
        <Text style={styles.description}>登录以继续使用 DevLoom</Text>

        <View style={styles.sheet}>
          <TextInput
            value={email}
            onChangeText={setEmail}
            placeholder="邮箱地址"
            placeholderTextColor="#9AA39B"
            autoCapitalize="none"
            autoCorrect={false}
            keyboardType="email-address"
            textContentType="emailAddress"
            editable={!busy}
            style={styles.input}
          />
          <View style={styles.passwordRow}>
            <TextInput
              value={password}
              onChangeText={setPassword}
              placeholder="密码"
              placeholderTextColor="#9AA39B"
              autoCapitalize="none"
              secureTextEntry={!showPassword}
              textContentType="password"
              editable={!busy}
              style={styles.passwordInput}
            />
            <Pressable onPress={() => setShowPassword((value) => !value)} disabled={busy} style={styles.passwordToggle}>
              <Text style={styles.toggleText}>{showPassword ? '隐藏' : '显示'}</Text>
            </Pressable>
          </View>

          <View style={styles.agreementRow}>
            <Pressable onPress={() => setAgreed((value) => !value)} style={[styles.checkbox, agreed && styles.checkboxActive]}>
              {agreed ? <Text style={styles.checkmark}>✓</Text> : null}
            </Pressable>
            <Text style={styles.agreement}>
              我已阅读并同意
              <Text onPress={() => openPolicy('/user-agreement')} style={styles.link}>《用户协议》</Text>
              和
              <Text onPress={() => openPolicy('/privacy-policy')} style={styles.link}>《隐私政策》</Text>
            </Text>
          </View>

          {error ? <Text style={styles.error}>{error}</Text> : null}

          <Pressable onPress={onLogin} disabled={busy} style={({ pressed }) => [styles.primaryButton, pressed && styles.pressed, busy && styles.disabled]}>
            {busy ? <ActivityIndicator color="#FFFFFF" /> : <Text style={styles.primaryText}>登录 DevLoom</Text>}
          </Pressable>

          {appleAvailable ? (
            <Pressable onPress={onAppleLogin} disabled={busy} style={({ pressed }) => [styles.secondaryButton, pressed && styles.pressed, busy && styles.disabled]}>
              <Text style={styles.secondaryText}>使用 Apple 登录</Text>
            </Pressable>
          ) : null}

          <Pressable onPress={() => setShowServer((value) => !value)} style={styles.serverToggle}>
            <Text style={styles.serverToggleText}>{showServer ? '收起服务配置' : '配置服务地址'}</Text>
          </Pressable>

          {showServer ? (
            <View style={styles.serverPanel}>
              <Text style={styles.serverLabel}>API 服务地址</Text>
              <TextInput value={serverUrl} onChangeText={setServerUrl} autoCapitalize="none" autoCorrect={false} placeholder={DEFAULT_BASE_URL} placeholderTextColor="#9AA39B" style={styles.input} />
              <Text style={styles.serverLabel}>HTTP Basic Auth（可选）</Text>
              <TextInput value={basicAuthInput} onChangeText={setBasicAuthInput} autoCapitalize="none" autoCorrect={false} placeholder="user:password" placeholderTextColor="#9AA39B" style={styles.input} />
            </View>
          ) : null}
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#0B1710' },
  content: { flexGrow: 1, paddingHorizontal: 24 },
  brandRow: { flexDirection: 'row', alignItems: 'center', gap: 14 },
  logo: { width: 58, height: 58, borderRadius: 16, backgroundColor: '#FFFFFF', padding: 8 },
  brand: { color: '#FFFFFF', fontSize: 27, fontWeight: '800' },
  subtitle: { color: '#B6D7BF', fontSize: 13, marginTop: 2 },
  title: { color: '#FFFFFF', fontSize: 30, fontWeight: '800', marginTop: 38 },
  description: { color: '#C6D8CB', fontSize: 15, marginTop: 8 },
  sheet: { backgroundColor: '#FFFFFF', borderRadius: 24, padding: 22, marginTop: 28, gap: 14 },
  input: { minHeight: 52, borderWidth: 1, borderColor: '#E1E6E1', borderRadius: 13, backgroundColor: '#F7F9F7', color: '#1D251F', paddingHorizontal: 14, fontSize: 15 },
  passwordRow: { minHeight: 52, flexDirection: 'row', alignItems: 'center', borderWidth: 1, borderColor: '#E1E6E1', borderRadius: 13, backgroundColor: '#F7F9F7' },
  passwordInput: { flex: 1, minHeight: 50, color: '#1D251F', paddingHorizontal: 14, fontSize: 15 },
  passwordToggle: { paddingHorizontal: 14, paddingVertical: 12 },
  toggleText: { color: '#278D4F', fontSize: 13, fontWeight: '700' },
  agreementRow: { flexDirection: 'row', alignItems: 'flex-start', gap: 9, marginTop: 2 },
  checkbox: { width: 20, height: 20, borderRadius: 6, borderWidth: 2, borderColor: '#C7D0C8', alignItems: 'center', justifyContent: 'center' },
  checkboxActive: { backgroundColor: '#278D4F', borderColor: '#278D4F' },
  checkmark: { color: '#FFFFFF', fontSize: 13, fontWeight: '800' },
  agreement: { flex: 1, color: '#7D887F', fontSize: 12.5, lineHeight: 19 },
  link: { color: '#278D4F', fontWeight: '700' },
  error: { color: '#C43E3E', fontSize: 13, lineHeight: 19 },
  primaryButton: { minHeight: 52, borderRadius: 13, alignItems: 'center', justifyContent: 'center', backgroundColor: '#278D4F' },
  primaryText: { color: '#FFFFFF', fontSize: 15, fontWeight: '800' },
  secondaryButton: { minHeight: 50, borderRadius: 13, borderWidth: 1, borderColor: '#D8E0D9', alignItems: 'center', justifyContent: 'center' },
  secondaryText: { color: '#1D251F', fontSize: 14, fontWeight: '700' },
  serverToggle: { alignItems: 'center', paddingVertical: 4 },
  serverToggleText: { color: '#647169', fontSize: 13, fontWeight: '600' },
  serverPanel: { borderTopWidth: 1, borderTopColor: '#E7ECE7', paddingTop: 14, gap: 8 },
  serverLabel: { color: '#5C6860', fontSize: 12, fontWeight: '700' },
  pressed: { opacity: 0.82 },
  disabled: { opacity: 0.55 },
});
