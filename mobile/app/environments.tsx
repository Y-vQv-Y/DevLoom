import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useRouter } from 'expo-router';
import { ActivityIndicator, Alert, Pressable, ScrollView, Text, TextInput, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { ApiError, createVirtualMachine, deleteVirtualMachine, listHosts, listImages, listModels } from '@/api/client';
import type { Host, Image, Model, VirtualMachine } from '@/api/types';
import { Card, IconButton, PickerSheet, PrimaryButton, type PickerOption } from '@/components/ui';
import { useTheme } from '@/theme';

function memoryLabel(bytes?: number) {
  if (!bytes) return '';
  return `${Math.round(bytes / (1024 * 1024 * 1024))} GB`;
}

function vmHostId(vm: VirtualMachine, hosts: Host[]) {
  return vm.host?.id || hosts.find((h) => h.virtualmachines?.some((item) => item.id === vm.id))?.id || '';
}

function isPrivateHost(host: Host) {
  return Boolean(host.id) && !host.id!.startsWith('public_host');
}

export default function EnvironmentsScreen() {
  const t = useTheme();
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const [hosts, setHosts] = useState<Host[]>([]);
  const [images, setImages] = useState<Image[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [hostId, setHostId] = useState('');
  const [imageId, setImageId] = useState('');
  const [modelId, setModelId] = useState('');
  const [picker, setPicker] = useState<'host' | 'image' | 'model' | null>(null);

  const refresh = useCallback(async () => {
    setError('');
    try {
      const [nextHosts, nextImages, nextModels] = await Promise.all([listHosts(), listImages(), listModels()]);
      const privateHosts = nextHosts.filter(isPrivateHost);
      setHosts(privateHosts);
      setImages(nextImages);
      setModels(nextModels);
      setHostId((current) => privateHosts.some((h) => h.id === current && h.status === 'online')
        ? current
        : privateHosts.find((h) => h.status === 'online' && (h.is_default || h.default))?.id || privateHosts.find((h) => h.status === 'online')?.id || '');
      setImageId((current) => current || nextImages.find((i) => i.is_default)?.id || nextImages[0]?.id || '');
      setModelId((current) => current || nextModels.find((m) => m.is_default)?.id || nextModels[0]?.id || '');
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Unable to load development environments');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);

  const vms = useMemo(() => hosts.flatMap((host) => (host.virtualmachines || []).map((vm) => ({ ...vm, host: vm.host || host }))), [hosts]);
  const hostOptions: PickerOption[] = hosts.map((host) => ({
    key: host.id || '',
    title: host.name || host.id || 'Host',
    sub: host.status || 'offline',
    icon: 'server',
    disabled: host.status !== 'online',
  }));
  const imageOptions: PickerOption[] = images.map((image) => ({ key: image.id || '', title: image.name || image.id || 'Image', sub: image.remark, icon: 'cube' }));
  const modelOptions: PickerOption[] = models.map((model) => ({ key: model.id || '', title: model.remark || model.model || model.id || 'Model', sub: model.provider, icon: 'brain' }));
  const selectedHost = hosts.find((host) => host.id === hostId);
  const selectedImage = images.find((image) => image.id === imageId);
  const selectedModel = models.find((model) => model.id === modelId);

  const create = useCallback(async () => {
    if (!name.trim() || !selectedHost || !isPrivateHost(selectedHost) || selectedHost.status !== 'online' || !imageId || !modelId) {
      setError('Enter a name and choose an online host, image, and model');
      return;
    }
    setCreating(true);
    setError('');
    try {
      await createVirtualMachine({
        name: name.trim(), host_id: hostId, image_id: imageId, model_id: modelId,
        life: 3 * 60 * 60, resource: { cpu: 2, memory: 8 * 1024 * 1024 * 1024 },
      });
      setName('');
      await refresh();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Unable to create the environment');
    } finally {
      setCreating(false);
    }
  }, [hostId, imageId, modelId, name, refresh, selectedHost]);

  const remove = (vm: VirtualMachine) => {
    const id = vm.id || '';
    const host = vmHostId(vm, hosts);
    if (!id || !host) return;
    Alert.alert('Delete environment', 'This removes the remote development environment.', [
      { text: 'Cancel', style: 'cancel' },
      { text: 'Delete', style: 'destructive', onPress: async () => {
        try { await deleteVirtualMachine(host, id); await refresh(); }
        catch (e) { setError(e instanceof ApiError ? e.message : 'Unable to delete the environment'); }
      } },
    ]);
  };

  return (
    <View style={{ flex: 1, backgroundColor: t.bg }}>
      <View style={{ paddingTop: insets.top + 8, paddingHorizontal: 12, height: insets.top + 60, flexDirection: 'row', alignItems: 'center' }}>
        <IconButton icon="back" onPress={() => router.back()} iconSize={22} />
        <Text style={{ flex: 1, textAlign: 'center', color: t.tx, fontSize: 18, fontWeight: '800' }}>Development environments</Text>
        <View style={{ width: 38 }} />
      </View>
      <ScrollView contentContainerStyle={{ padding: 16, paddingBottom: insets.bottom + 40 }}>
        <Card style={{ padding: 15, marginBottom: 14 }}>
          <Text style={{ color: t.tx, fontSize: 16, fontWeight: '800' }}>Create an environment</Text>
          <TextInput value={name} onChangeText={setName} placeholder="Environment name" placeholderTextColor={t.tx3} style={{ marginTop: 12, borderWidth: 1, borderColor: t.line, borderRadius: 10, padding: 12, color: t.tx }} />
          <Pressable onPress={() => setPicker('host')} style={{ marginTop: 10, padding: 12, borderWidth: 1, borderColor: t.line, borderRadius: 10 }}><Text style={{ color: t.tx }}>{selectedHost?.name || 'Choose host'}</Text></Pressable>
          <Pressable onPress={() => setPicker('image')} style={{ marginTop: 10, padding: 12, borderWidth: 1, borderColor: t.line, borderRadius: 10 }}><Text style={{ color: t.tx }}>{selectedImage?.name || 'Choose image'}</Text></Pressable>
          <Pressable onPress={() => setPicker('model')} style={{ marginTop: 10, padding: 12, borderWidth: 1, borderColor: t.line, borderRadius: 10 }}><Text style={{ color: t.tx }}>{selectedModel?.remark || selectedModel?.model || 'Choose model'}</Text></Pressable>
          <PrimaryButton block icon="plus" label={creating ? 'Creating...' : 'Create'} disabled={creating} onPress={create} style={{ marginTop: 12 }} />
        </Card>
        {loading ? <ActivityIndicator color={t.ac} /> : null}
        {vms.map((vm) => (
          <Card key={vm.id || vm.environment_id || vm.hostname || vm.name || String(vm.created_at || 'vm')} style={{ padding: 15, marginBottom: 10 }}>
            <View style={{ flexDirection: 'row', alignItems: 'center' }}>
              <View style={{ flex: 1 }}><Text style={{ color: t.tx, fontSize: 15, fontWeight: '700' }}>{vm.name || vm.hostname || vm.id}</Text><Text style={{ color: t.tx3, marginTop: 4 }}>{vm.os || 'Linux'} · {memoryLabel(vm.memory)} · {vm.status || 'unknown'}</Text></View>
              <IconButton icon="trash" onPress={() => remove(vm)} iconSize={18} />
            </View>
            {vm.id ? <PrimaryButton block icon="terminal" label="Open terminal" onPress={() => router.push(`/environment-terminal?id=${encodeURIComponent(vm.id || '')}`)} /> : null}
          </Card>
        ))}
        {!loading && vms.length === 0 ? <Text style={{ color: t.tx3, textAlign: 'center', marginTop: 20 }}>No development environments</Text> : null}
        {error ? <Text style={{ color: t.red, marginTop: 12 }}>{error}</Text> : null}
      </ScrollView>
      <PickerSheet visible={picker === 'host'} title="Choose host" options={hostOptions} selected={hostId} onPick={(key) => { setHostId(key); setPicker(null); }} onClose={() => setPicker(null)} />
      <PickerSheet visible={picker === 'image'} title="Choose image" options={imageOptions} selected={imageId} onPick={(key) => { setImageId(key); setPicker(null); }} onClose={() => setPicker(null)} />
      <PickerSheet visible={picker === 'model'} title="Choose model" options={modelOptions} selected={modelId} onPick={(key) => { setModelId(key); setPicker(null); }} onClose={() => setPicker(null)} />
    </View>
  );
}
