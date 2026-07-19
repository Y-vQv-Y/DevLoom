import { useLocalSearchParams, useRouter } from 'expo-router';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, Text, TextInput, View, ScrollView } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { base64DecodeToString, base64Encode } from '@/messages/base64';
import { getBaseUrl, openWebSocket } from '@/api/client';
import { Icons } from '@/components/Icons';

type TerminalMessage = { type?: string; data?: string };

function createTerminalId() {
  return `mobile-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

export default function EnvironmentTerminalScreen() {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { id } = useLocalSearchParams<{ id: string }>();
  const [output, setOutput] = useState('');
  const [input, setInput] = useState('');
  const [connected, setConnected] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const terminalIdRef = useRef(createTerminalId());
  const socketRef = useRef<WebSocket | null>(null);

  const connect = useCallback(() => {
    if (!id) return;
    socketRef.current?.close();
    const base = getBaseUrl().replace(/^http:/, 'ws:').replace(/^https:/, 'wss:');
    const url = `${base}/api/v1/users/hosts/vms/${encodeURIComponent(id)}/terminals/connect?terminal_id=${encodeURIComponent(terminalIdRef.current)}&col=100&row=30`;
    setConnecting(true);
    setConnected(false);
    try {
      const socket = openWebSocket(url);
      socketRef.current = socket;
      socket.onopen = () => {
        if (socketRef.current !== socket) return;
        setConnecting(false);
        setConnected(true);
      };
      socket.onmessage = (event: MessageEvent<string>) => {
        let message: TerminalMessage;
        try { message = JSON.parse(String(event.data)); } catch { return; }
        if (message.type === 'ping') {
          socket.send(JSON.stringify({ type: 'pong' }));
        } else if (message.type === 'data' && message.data) {
          try { setOutput((current) => (current + base64DecodeToString(message.data!)).slice(-100000)); } catch { /* ignore malformed data */ }
        } else if (message.type === 'error') {
          setOutput((current) => `${current}\n[error] ${message.data || 'terminal error'}\n`);
        }
      };
      socket.onerror = () => {
        if (socketRef.current !== socket) return;
        setConnecting(false);
        setConnected(false);
      };
      socket.onclose = () => {
        if (socketRef.current !== socket) return;
        socketRef.current = null;
        setConnecting(false);
        setConnected(false);
      };
    } catch {
      setConnecting(false);
      setConnected(false);
    }
  }, [id]);

  useEffect(() => {
    connect();
    return () => { socketRef.current?.close(); socketRef.current = null; };
  }, [connect]);

  const send = useCallback((value: string) => {
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN || !value) return;
    socket.send(JSON.stringify({ type: 'data', data: base64Encode(value) }));
  }, []);
  const submit = () => { if (input) { send(`${input}\n`); setInput(''); } };

  return (
    <View style={{ flex: 1, backgroundColor: '#101416', paddingTop: insets.top + 8 }}>
      <View style={{ height: 52, flexDirection: 'row', alignItems: 'center', paddingHorizontal: 10 }}>
        <Pressable onPress={() => router.back()} style={{ padding: 8 }}><Icons.back size={22} color="#dce7e5" /></Pressable>
        <Text style={{ flex: 1, textAlign: 'center', color: '#eaf4f1', fontSize: 16, fontWeight: '700' }}>Remote terminal</Text>
        <Pressable onPress={connect} style={{ padding: 8 }}><Icons.refresh size={20} color={connected ? '#8ce2ba' : '#dce7e5'} /></Pressable>
      </View>
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 7, paddingHorizontal: 16, paddingBottom: 8 }}>
        {connecting ? <ActivityIndicator size="small" color="#8ce2ba" /> : <View style={{ width: 8, height: 8, borderRadius: 99, backgroundColor: connected ? '#8ce2ba' : '#f08080' }} />}
        <Text style={{ color: '#9eb2ae', fontSize: 12 }}>{connecting ? 'Connecting...' : connected ? 'Connected' : 'Disconnected'}</Text>
      </View>
      <ScrollView style={{ flex: 1, margin: 12, padding: 12, borderRadius: 10, backgroundColor: '#080b0c' }} contentContainerStyle={{ paddingBottom: 20 }}>
        <Text selectable style={{ color: '#d6e5df', fontFamily: 'monospace', fontSize: 12, lineHeight: 18 }}>{output || 'Terminal output will appear here.'}</Text>
      </ScrollView>
      <View style={{ flexDirection: 'row', gap: 8, padding: 12, paddingBottom: Math.max(insets.bottom, 12), borderTopWidth: 1, borderColor: '#253130' }}>
        <TextInput value={input} onChangeText={setInput} onSubmitEditing={submit} returnKeyType="send" editable={connected} placeholder="Type a command" placeholderTextColor="#71827e" style={{ flex: 1, color: '#eaf4f1', backgroundColor: '#1a2222', borderRadius: 9, paddingHorizontal: 12, paddingVertical: 10, fontFamily: 'monospace' }} />
        <Pressable onPress={() => send('\u0003')} disabled={!connected} style={{ justifyContent: 'center', paddingHorizontal: 10, borderRadius: 9, backgroundColor: '#392425' }}><Text style={{ color: '#ffaaaa', fontWeight: '700' }}>Ctrl-C</Text></Pressable>
        <Pressable onPress={submit} disabled={!connected || !input} style={{ justifyContent: 'center', paddingHorizontal: 13, borderRadius: 9, backgroundColor: connected && input ? '#8ce2ba' : '#394541' }}><Text style={{ color: connected && input ? '#102019' : '#9aa9a4', fontWeight: '700' }}>Send</Text></Pressable>
      </View>
    </View>
  );
}
