import { useNavigate, useSearchParams } from 'react-router-dom';
import Container from '../components/Container';
import JuryHeader from '../components/JuryHeader';
import { useEffect, useState } from 'react';
import Loading from '../components/Loading';
import TextInput from '../components/TextInput';
import Button from '../components/Button';
import Cookies from 'universal-cookie';
import { postRequest } from '../api';
import { errorAlert } from '../util';

const AddSelf = () => {
    const [searchParams, _] = useSearchParams({ track: '' });
    const navigate = useNavigate();
    const [loaded, setLoaded] = useState(false);
    const [code, setCode] = useState('');
    const [track, setTrack] = useState('');
    const [name, setName] = useState('');
    const [registrationError, setRegistrationError] = useState('');

    useEffect(() => {
        async function checkCode() {
            const cookies = new Cookies();
            if (cookies.get('token')) {
                const authRes = await postRequest<OkResponse>('/judge/auth', 'judge', null);
                if (authRes.status === 200 && authRes.data?.ok === 1) {
                    navigate('/judge', { replace: true });
                    return;
                }
                if (authRes.status !== 401) {
                    errorAlert(authRes);
                    setRegistrationError('Could not check your session. Please try again.');
                    setLoaded(true);
                    return;
                }
                cookies.remove('token', { path: '/' });
            }
            // Get track
            const tr = searchParams.get('track') ?? '';

            // Get code
            const paramCode = searchParams.get('code');
            if (!paramCode) {
                setRegistrationError('Code not found. Re-scan the QR code or ask an organizer.');
                setLoaded(true);
                return;
            }

            // Get QR code
            let correctCode;
            if (tr !== '') {
                const res = await postRequest<OkResponse>(`/qr/check/${encodeURIComponent(tr)}`, '', {
                    code: paramCode,
                });
                if (res.status !== 200) {
                    errorAlert(res);
                    setRegistrationError('Could not check the QR code. Please try again.');
                    setLoaded(true);
                    return;
                }

                correctCode = res.data?.ok;
            } else {
                const res = await postRequest<OkResponse>('/qr/check', '', { code: paramCode });
                if (res.status !== 200) {
                    errorAlert(res);
                    setRegistrationError('Could not check the QR code. Please try again.');
                    setLoaded(true);
                    return;
                }
                correctCode = res.data?.ok;
            }

            // Check code
            if (!correctCode) {
                setRegistrationError('Invalid code. Re-scan the QR code or ask an organizer.');
                setLoaded(true);
                return;
            }

            setCode(paramCode);
            setTrack(tr);
            setLoaded(true);
        }

        checkCode();
    }, []);

    const createJudge = async () => {
        if (!loaded || !code) return;
        if (!name.trim()) {
            alert('Please enter your name.');
            return;
        }
        setLoaded(false);

        const res = await postRequest<TokenResponse>('/qr/add', '', {
            name: name.trim(),
            track,
            code,
        });
        if (res.status !== 200) {
            errorAlert(res);
            setLoaded(true);
            return;
        }

        if (!res.data?.token) {
            alert('Could not sign in. Please try again or contact an organizer.');
            setLoaded(true);
            return;
        }
        const cookies = new Cookies();
        cookies.set('token', res.data.token, {
            path: '/',
            sameSite: 'strict',
            secure: window.location.protocol === 'https:',
            maxAge: 60 * 60 * 24,
        });
        navigate('/judge/welcome', { replace: true });

        setLoaded(true);
    };

    return (
        <>
            <JuryHeader />
            <Container>
                <h1 className="text-3xl">Add Judge Form</h1>
                <p className="text-light mt-2 mb-8 px-4 text-center">
                    Enter your name to join {track || 'general'} judging. You’ll be signed in
                    immediately and taken to the judging instructions.
                </p>
                <TextInput text={name} setText={setName} label="Name" large />
                {registrationError && (
                    <p className="text-error text-center mt-4">{registrationError}</p>
                )}
                <Button
                    type="primary"
                    onClick={createJudge}
                    disabled={!loaded || !code || !name.trim()}
                    className="mt-8"
                >
                    Submit
                </Button>
                <Loading disabled={loaded} />
            </Container>
        </>
    );
};

export default AddSelf;
